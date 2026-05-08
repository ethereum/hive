use std::collections::{HashMap, HashSet};
use std::io::{self, Cursor, Write};
use std::time::Duration;

use alloy_primitives::B256;
use async_trait::async_trait;
use futures::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt};
use futures::prelude::*;
use libp2p::{
    gossipsub::{
        Behaviour as GossipsubBehaviour, Config as GossipsubConfig, Event as GossipsubEvent,
        IdentTopic, MessageAuthenticity, TopicHash,
    },
    request_response::{
        self, Behaviour as RequestResponseBehaviour, Codec, Event as ReqRespEvent,
        Message as ReqRespMessage, OutboundRequestId, ProtocolSupport, ResponseChannel,
    },
    swarm::{NetworkBehaviour, SwarmEvent},
    Multiaddr, PeerId, StreamProtocol, Swarm,
};
use libp2p_identity::secp256k1::SecretKey;
use sha2::{Digest, Sha256};
use snap::{read::FrameDecoder, write::FrameEncoder};
use ssz::Encode;
use ssz_derive::{Decode as SszDecodeDerive, Encode as SszEncodeDerive};
use ssz_types::{
    typenum::{U1024, U1048576, U4096},
    BitList, VariableList,
};
use tokio::time::timeout;

// Protocol strings for lean reqresp
pub const LEAN_STATUS_PROTOCOL: &str = "/leanconsensus/req/status/1/ssz_snappy";
pub const LEAN_BLOCKS_BY_ROOT_PROTOCOL: &str = "/leanconsensus/req/blocks_by_root/1/ssz_snappy";

// Gossip topic helpers
pub fn lean_block_topic(fork_digest: &str) -> IdentTopic {
    IdentTopic::new(format!("/leanconsensus/{fork_digest}/block/ssz_snappy"))
}

pub fn lean_attestation_topic(fork_digest: &str, subnet_id: u64) -> IdentTopic {
    IdentTopic::new(format!(
        "/leanconsensus/{fork_digest}/attestation_{subnet_id}/ssz_snappy"
    ))
}

pub fn lean_aggregation_topic(fork_digest: &str) -> IdentTopic {
    IdentTopic::new(format!(
        "/leanconsensus/{fork_digest}/aggregated_attestation/ssz_snappy"
    ))
}

// Response codes
pub const RESPONSE_CODE_SUCCESS: u8 = 0;
pub const RESPONSE_CODE_INVALID_REQUEST: u8 = 1;
pub const RESPONSE_CODE_SERVER_ERROR: u8 = 2;
pub const RESPONSE_CODE_RESOURCE_UNAVAILABLE: u8 = 3;

/// Maximum number of block roots in a BlocksByRoot request.
pub const MAX_REQUEST_BLOCKS: usize = 1024;

// === SSZ Types ===

#[derive(Debug, Default, Clone, Copy, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct Checkpoint {
    pub root: B256,
    pub slot: u64,
}

#[derive(Debug, Default, Clone, Copy, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct Status {
    pub finalized: Checkpoint,
    pub head: Checkpoint,
}

impl Status {
    pub fn from_ssz_bytes(bytes: &[u8]) -> Result<Self, ssz::DecodeError> {
        ssz::Decode::from_ssz_bytes(bytes)
    }
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct BlocksByRootV1Request {
    pub roots: VariableList<B256, U1024>,
}

impl BlocksByRootV1Request {
    pub fn new(roots: Vec<B256>) -> Self {
        Self {
            roots: VariableList::new(roots).expect("too many roots"),
        }
    }

    pub fn from_ssz_bytes(bytes: &[u8]) -> Result<Self, ssz::DecodeError> {
        ssz::Decode::from_ssz_bytes(bytes)
    }
}

// === Block / Attestation SSZ Types ===

/// 3112-byte post-quantum signature placeholder.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LeanSignature(pub [u8; 3112]);

impl Default for LeanSignature {
    fn default() -> Self {
        Self([0u8; 3112])
    }
}

impl ssz::Encode for LeanSignature {
    fn is_ssz_fixed_len() -> bool {
        true
    }
    fn ssz_fixed_len() -> usize {
        3112
    }
    fn ssz_append(&self, buf: &mut Vec<u8>) {
        buf.extend_from_slice(&self.0);
    }
    fn ssz_bytes_len(&self) -> usize {
        3112
    }
}

impl ssz::Decode for LeanSignature {
    fn is_ssz_fixed_len() -> bool {
        true
    }
    fn ssz_fixed_len() -> usize {
        3112
    }
    fn from_ssz_bytes(bytes: &[u8]) -> Result<Self, ssz::DecodeError> {
        if bytes.len() != 3112 {
            return Err(ssz::DecodeError::InvalidByteLength {
                len: bytes.len(),
                expected: 3112,
            });
        }
        let mut arr = [0u8; 3112];
        arr.copy_from_slice(bytes);
        Ok(Self(arr))
    }
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanAttestationData {
    pub slot: u64,
    pub head: Checkpoint,
    pub target: Checkpoint,
    pub source: Checkpoint,
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanSignedAttestation {
    pub validator_id: u64,
    pub data: LeanAttestationData,
    pub signature: LeanSignature,
}

impl LeanSignedAttestation {
    /// Build a signed attestation with the given data and a zero signature.
    pub fn build_unsigned(validator_id: u64, data: LeanAttestationData) -> Self {
        Self {
            validator_id,
            data,
            signature: LeanSignature::default(),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanAggregatedAttestation {
    pub aggregation_bits: BitList<U4096>,
    pub message: LeanAttestationData,
}

#[derive(Debug, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanAggregatedSignatureProof {
    pub participants: BitList<U4096>,
    pub proof_data: VariableList<u8, U1048576>,
    pub bytecode_point: Option<VariableList<u8, U1048576>>,
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanBlockBody {
    pub attestations: VariableList<LeanAggregatedAttestation, U4096>,
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanBlock {
    pub slot: u64,
    pub proposer_index: u64,
    pub parent_root: B256,
    pub state_root: B256,
    pub body: LeanBlockBody,
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanBlockSignatures {
    pub attestation_signatures: VariableList<LeanAggregatedSignatureProof, U4096>,
    pub proposer_signature: LeanSignature,
}

#[derive(Debug, Default, Clone, PartialEq, Eq, SszEncodeDerive, SszDecodeDerive)]
pub struct LeanSignedBlock {
    pub block: LeanBlock,
    pub signature: LeanBlockSignatures,
}

impl LeanSignedBlock {
    /// Build a minimal valid-SSZ signed block for the given slot with the specified parent.
    pub fn build_minimal(
        slot: u64,
        proposer_index: u64,
        parent_root: B256,
        state_root: B256,
    ) -> Self {
        Self {
            block: LeanBlock {
                slot,
                proposer_index,
                parent_root,
                state_root,
                body: LeanBlockBody {
                    attestations: VariableList::new(vec![]).expect("empty attestation list"),
                },
            },
            signature: LeanBlockSignatures {
                attestation_signatures: VariableList::new(vec![]).expect("empty signature list"),
                proposer_signature: LeanSignature::default(),
            },
        }
    }
}

/// Build a response chunk for a successful BlocksByRoot response.
pub fn build_block_response_chunk(block: &LeanSignedBlock) -> Vec<u8> {
    let ssz_bytes = block.as_ssz_bytes();
    let mut encoder = FrameEncoder::new(Vec::new());
    encoder.write_all(&ssz_bytes).unwrap();
    encoder.flush().unwrap();
    let compressed = encoder.into_inner().unwrap();

    let mut result = encode_varint_usize(ssz_bytes.len());
    result.extend_from_slice(&compressed);
    result
}

/// Encode data for gossipsub: snappy-compressed SSZ bytes (no varint prefix).
pub fn encode_gossip_data<T: Encode>(item: &T) -> Vec<u8> {
    let ssz_bytes = item.as_ssz_bytes();
    let mut encoder = FrameEncoder::new(Vec::new());
    encoder.write_all(&ssz_bytes).unwrap();
    encoder.flush().unwrap();
    encoder.into_inner().unwrap()
}

// === Peer ID Computation ===

/// Compute the deterministic PeerId for a lean client based on its name.
/// This matches the logic in `prepare_lean_client_assets.py`:
///   key = sha256(f"{client_kind}:{node_id}:node").hexdigest()
/// where NODE_ID defaults to "{client_kind}_0".
pub fn compute_client_peer_id(client_name: &str) -> PeerId {
    let base_name = client_name.split('_').next().unwrap_or(client_name);
    let label = format!("{}:{}_0:node", base_name, base_name);
    let hash = Sha256::digest(label.as_bytes());
    let mut key_bytes = hash.to_vec();

    let secret_key = SecretKey::try_from_bytes(&mut key_bytes)
        .expect("Failed to create secp256k1 secret key from deterministic bytes");
    let secp_keypair = libp2p_identity::secp256k1::Keypair::from(secret_key);
    let keypair = libp2p_identity::Keypair::from(secp_keypair);
    keypair.public().to_peer_id()
}

/// Build the client's multiaddr from its IP.
pub fn client_multiaddr(ip: std::net::IpAddr, port: u16) -> Multiaddr {
    let mut addr = Multiaddr::empty();
    match ip {
        std::net::IpAddr::V4(ip) => addr.push(libp2p::multiaddr::Protocol::Ip4(ip)),
        std::net::IpAddr::V6(ip) => addr.push(libp2p::multiaddr::Protocol::Ip6(ip)),
    }
    addr.push(libp2p::multiaddr::Protocol::Udp(port));
    addr.push(libp2p::multiaddr::Protocol::QuicV1);
    addr
}

/// Extract the IP and UDP port from a multiaddr.
pub fn extract_ip_port(addr: &Multiaddr) -> Option<(std::net::IpAddr, u16)> {
    let mut ip = None;
    let mut port = None;
    for proto in addr.iter() {
        match proto {
            libp2p::multiaddr::Protocol::Ip4(v4) => ip = Some(std::net::IpAddr::V4(v4)),
            libp2p::multiaddr::Protocol::Ip6(v6) => ip = Some(std::net::IpAddr::V6(v6)),
            libp2p::multiaddr::Protocol::Udp(p) => port = Some(p),
            _ => {}
        }
    }
    ip.zip(port)
}

/// Replace the IP address in a multiaddr with a new one, preserving the rest.
pub fn replace_multiaddr_ip(addr: &Multiaddr, ip: std::net::IpAddr) -> Multiaddr {
    let mut new_addr = Multiaddr::empty();
    for proto in addr.iter() {
        match proto {
            libp2p::multiaddr::Protocol::Ip4(_) | libp2p::multiaddr::Protocol::Ip6(_) => match ip {
                std::net::IpAddr::V4(v4) => new_addr.push(libp2p::multiaddr::Protocol::Ip4(v4)),
                std::net::IpAddr::V6(v6) => new_addr.push(libp2p::multiaddr::Protocol::Ip6(v6)),
            },
            other => new_addr.push(other),
        }
    }
    new_addr
}

// === Encoding / Decoding ===

fn encode_varint_usize(value: usize) -> Vec<u8> {
    let mut buf = unsigned_varint::encode::usize_buffer();
    unsigned_varint::encode::usize(value, &mut buf).to_vec()
}

fn decode_varint_usize(buf: &[u8]) -> io::Result<(usize, &[u8])> {
    unsigned_varint::decode::usize(buf).map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))
}

/// Encode SSZ bytes + snappy compression for a request payload.
pub fn encode_request<T: Encode>(item: &T) -> Vec<u8> {
    let ssz_bytes = item.as_ssz_bytes();
    let mut encoder = FrameEncoder::new(Vec::new());
    encoder.write_all(&ssz_bytes).unwrap();
    encoder.flush().unwrap();
    let compressed = encoder.into_inner().unwrap();

    let mut result = encode_varint_usize(ssz_bytes.len());
    result.extend_from_slice(&compressed);
    result
}

/// Encode raw bytes with snappy compression for a request payload.
pub fn encode_request_raw(bytes: &[u8]) -> Vec<u8> {
    let mut encoder = FrameEncoder::new(Vec::new());
    encoder.write_all(bytes).unwrap();
    encoder.flush().unwrap();
    let compressed = encoder.into_inner().unwrap();

    let mut result = encode_varint_usize(bytes.len());
    result.extend_from_slice(&compressed);
    result
}

/// Decode a request payload from raw bytes (varint length prefix + snappy).
pub fn decode_request(data: &[u8]) -> io::Result<Vec<u8>> {
    let (length, rest) = decode_varint_usize(data)?;
    let mut decoder = FrameDecoder::new(Cursor::new(rest));
    let mut decompressed = vec![0u8; length];
    std::io::Read::read_exact(&mut decoder, &mut decompressed)?;
    Ok(decompressed)
}

/// Decode response chunks from a byte buffer.
/// Each chunk: response_code(1) + uvi(length) + snappy_data
pub fn decode_response_chunks(data: &[u8]) -> io::Result<Vec<(u8, Vec<u8>)>> {
    let mut cursor = Cursor::new(data);
    let mut chunks = Vec::new();

    while cursor.position() < data.len() as u64 {
        let mut code_buf = [0u8; 1];
        std::io::Read::read_exact(&mut cursor, &mut code_buf)?;
        let code = code_buf[0];

        let pos = cursor.position() as usize;
        let remaining = &data[pos..];
        let (length, rest) = decode_varint_usize(remaining)?;

        let compressed_start = pos + (remaining.len() - rest.len());
        let compressed_data = &data[compressed_start..];

        let mut decoder = FrameDecoder::new(Cursor::new(compressed_data));
        let mut decompressed = vec![0u8; length];
        std::io::Read::read_exact(&mut decoder, &mut decompressed)?;
        let consumed = decoder.get_ref().position() as usize;

        cursor.set_position((compressed_start + consumed) as u64);
        chunks.push((code, decompressed));
    }

    Ok(chunks)
}

// === Lean ReqResp Codec ===

#[derive(Clone, Debug)]
pub struct LeanReqRespCodec;

#[async_trait]
impl Codec for LeanReqRespCodec {
    type Protocol = StreamProtocol;
    type Request = Vec<u8>;
    type Response = Vec<(u8, Vec<u8>)>;

    async fn read_request<T>(
        &mut self,
        _protocol: &Self::Protocol,
        io: &mut T,
    ) -> io::Result<Self::Request>
    where
        T: AsyncRead + Unpin + Send,
    {
        let mut buf = Vec::new();
        io.read_to_end(&mut buf).await?;
        Ok(buf)
    }

    async fn read_response<T>(
        &mut self,
        _protocol: &Self::Protocol,
        io: &mut T,
    ) -> io::Result<Self::Response>
    where
        T: AsyncRead + Unpin + Send,
    {
        let mut buf = Vec::new();
        io.read_to_end(&mut buf).await?;
        decode_response_chunks(&buf)
    }

    async fn write_request<T>(
        &mut self,
        _protocol: &Self::Protocol,
        io: &mut T,
        req: Self::Request,
    ) -> io::Result<()>
    where
        T: AsyncWrite + Unpin + Send,
    {
        io.write_all(&req).await?;
        io.flush().await?;
        Ok(())
    }

    async fn write_response<T>(
        &mut self,
        _protocol: &Self::Protocol,
        io: &mut T,
        res: Self::Response,
    ) -> io::Result<()>
    where
        T: AsyncWrite + Unpin + Send,
    {
        for (code, data) in res {
            io.write_all(&[code]).await?;
            io.write_all(&encode_varint_usize(data.len())).await?;

            let mut encoder = FrameEncoder::new(Vec::new());
            encoder.write_all(&data).unwrap();
            encoder.flush().unwrap();
            io.write_all(&encoder.into_inner().unwrap()).await?;
        }
        io.flush().await?;
        Ok(())
    }
}

// === Mock Node ===

#[derive(NetworkBehaviour)]
pub struct MockBehaviour {
    pub reqresp: RequestResponseBehaviour<LeanReqRespCodec>,
    pub gossipsub: GossipsubBehaviour,
    pub identify: libp2p::identify::Behaviour,
    pub ping: libp2p::ping::Behaviour,
}

pub struct MockNode {
    pub swarm: Swarm<MockBehaviour>,
    pending_requests: HashMap<
        OutboundRequestId,
        tokio::sync::oneshot::Sender<Result<Vec<(u8, Vec<u8>)>, String>>,
    >,
    gossip_messages: Vec<(PeerId, IdentTopic, Vec<u8>)>,
    secret_key_bytes: Option<[u8; 32]>,
    /// Gossipsub Subscribed events captured while draining events for other
    /// purposes (e.g. during `wait_for_request`). Tests that look for a peer
    /// subscription check this set first so they don't miss events that
    /// arrived before the test loop started.
    pub seen_subscriptions: HashMap<TopicHash, HashSet<PeerId>>,
}

impl MockNode {
    pub fn with_protocols(
        protocols: Vec<(StreamProtocol, ProtocolSupport)>,
    ) -> Result<Self, String> {
        let mut secret_key_bytes = [0u8; 32];
        rand::RngCore::fill_bytes(&mut rand::thread_rng(), &mut secret_key_bytes);
        let mut key_bytes = secret_key_bytes.to_vec();
        let secret_key = SecretKey::try_from_bytes(&mut key_bytes)
            .expect("Failed to create secp256k1 secret key from bytes");
        let secp_keypair = libp2p_identity::secp256k1::Keypair::from(secret_key);
        let keypair = libp2p_identity::Keypair::from(secp_keypair);

        let mut swarm = libp2p::SwarmBuilder::with_existing_identity(keypair.clone())
            .with_tokio()
            .with_quic()
            .with_behaviour(|_| {
                let cfg = request_response::Config::default();
                let gossipsub_config = GossipsubConfig::default();
                let gossipsub = GossipsubBehaviour::new(
                    MessageAuthenticity::Signed(keypair.clone()),
                    gossipsub_config,
                )
                .expect("Failed to create gossipsub behaviour");
                let identify_config = libp2p::identify::Config::new(
                    "/leanconsensus/identify/1.0.0".to_string(),
                    keypair.public(),
                )
                .with_agent_version("mock-node/0.1.0".to_string());
                MockBehaviour {
                    reqresp: RequestResponseBehaviour::with_codec(LeanReqRespCodec, protocols, cfg),
                    gossipsub,
                    identify: libp2p::identify::Behaviour::new(identify_config),
                    ping: libp2p::ping::Behaviour::new(libp2p::ping::Config::new()),
                }
            })
            .map_err(|e| format!("Failed to create swarm behaviour: {e}"))?
            .with_swarm_config(|c| c.with_idle_connection_timeout(Duration::from_secs(60)))
            .build();

        swarm
            .listen_on("/ip4/0.0.0.0/udp/0/quic-v1".parse().unwrap())
            .map_err(|e| format!("Failed to listen: {e}"))?;

        Ok(MockNode {
            swarm,
            pending_requests: HashMap::new(),
            gossip_messages: Vec::new(),
            secret_key_bytes: Some(secret_key_bytes),
            seen_subscriptions: HashMap::new(),
        })
    }

    pub fn new_status_only() -> Result<Self, String> {
        Self::with_protocols(vec![(
            StreamProtocol::new(LEAN_STATUS_PROTOCOL),
            ProtocolSupport::Full,
        )])
    }

    pub fn new_blocks_by_root_only() -> Result<Self, String> {
        Self::with_protocols(vec![(
            StreamProtocol::new(LEAN_BLOCKS_BY_ROOT_PROTOCOL),
            ProtocolSupport::Full,
        )])
    }

    pub fn new() -> Result<Self, String> {
        Self::with_protocols(vec![
            (
                StreamProtocol::new(LEAN_STATUS_PROTOCOL),
                ProtocolSupport::Full,
            ),
            (
                StreamProtocol::new(LEAN_BLOCKS_BY_ROOT_PROTOCOL),
                ProtocolSupport::Full,
            ),
        ])
    }

    pub fn local_peer_id(&self) -> PeerId {
        *self.swarm.local_peer_id()
    }

    /// Generate an Ethereum Node Record (ENR) for this mock node.
    /// The ENR encodes the IP, UDP port, and secp256k1 public key so that
    /// clients that only understand ENR bootnodes (e.g. ethlambda) can dial
    /// the mock node.
    pub fn enr_string(&self, ip: std::net::Ipv4Addr, udp_port: u16) -> Option<String> {
        let secret_bytes = self.secret_key_bytes.as_ref()?;
        let signing_key = enr::k256::ecdsa::SigningKey::from_slice(secret_bytes).ok()?;
        let enr = enr::Enr::<_>::builder()
            .ip4(ip)
            .udp4(udp_port)
            .add_value("quic", &udp_port)
            .build(&signing_key)
            .ok()?;
        Some(enr.to_base64())
    }

    pub fn dial(&mut self, peer_id: PeerId, addr: Multiaddr) -> Result<(), String> {
        self.swarm.add_peer_address(peer_id, addr.clone());
        let full_addr = addr.with(libp2p::multiaddr::Protocol::P2p(peer_id));
        self.swarm
            .dial(full_addr)
            .map_err(|e| format!("Dial failed: {e}"))
    }

    pub async fn send_request(
        &mut self,
        peer_id: PeerId,
        request: Vec<u8>,
    ) -> Result<Vec<(u8, Vec<u8>)>, String> {
        let (tx, _rx) = tokio::sync::oneshot::channel();
        let request_id = self
            .swarm
            .behaviour_mut()
            .reqresp
            .send_request(&peer_id, request);
        self.pending_requests.insert(request_id, tx);

        let deadline = tokio::time::Instant::now() + Duration::from_secs(30);

        loop {
            let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
            if remaining.is_zero() {
                self.pending_requests.remove(&request_id);
                return Err("Request timeout".to_string());
            }

            match timeout(remaining, self.swarm.select_next_some()).await {
                Ok(event) => match event {
                    SwarmEvent::Behaviour(MockBehaviourEvent::Reqresp(ReqRespEvent::Message {
                        peer: _,
                        message:
                            ReqRespMessage::Response {
                                request_id: rid,
                                response,
                            },
                        ..
                    })) => {
                        if rid == request_id {
                            self.pending_requests.remove(&request_id);
                            return Ok(response);
                        }
                    }
                    SwarmEvent::Behaviour(MockBehaviourEvent::Reqresp(
                        ReqRespEvent::OutboundFailure {
                            request_id: rid,
                            error,
                            ..
                        },
                    )) => {
                        if rid == request_id {
                            self.pending_requests.remove(&request_id);
                            return Err(format!("Outbound failure: {error:?}"));
                        }
                    }
                    _ => {}
                },
                Err(_) => {
                    self.pending_requests.remove(&request_id);
                    return Err("Request timeout".to_string());
                }
            }
        }
    }

    /// Wait for an incoming request and return it along with the response channel.
    ///
    /// Gossipsub `Subscribed` events that arrive while waiting are buffered
    /// into `self.seen_subscriptions` so that tests can check them
    /// retroactively.  Without this, clients that send their gossipsub
    /// SUBSCRIBE before the reqresp Status handshake completes would have
    /// their subscription silently dropped.
    pub async fn wait_for_request(
        &mut self,
    ) -> Option<(
        PeerId,
        request_response::InboundRequestId,
        Vec<u8>,
        ResponseChannel<Vec<(u8, Vec<u8>)>>,
    )> {
        loop {
            match self.swarm.next().await {
                Some(SwarmEvent::Behaviour(MockBehaviourEvent::Reqresp(
                    ReqRespEvent::Message {
                        peer,
                        message:
                            ReqRespMessage::Request {
                                request_id,
                                request,
                                channel,
                            },
                        ..
                    },
                ))) => {
                    return Some((peer, request_id, request, channel));
                }
                Some(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(
                    GossipsubEvent::Subscribed { peer_id, topic },
                ))) => {
                    self.seen_subscriptions
                        .entry(topic)
                        .or_default()
                        .insert(peer_id);
                }
                Some(_) => continue,
                None => return None,
            }
        }
    }

    /// Send a response to an inbound request.
    pub fn send_response(
        &mut self,
        channel: ResponseChannel<Vec<(u8, Vec<u8>)>>,
        response: Vec<(u8, Vec<u8>)>,
    ) -> Result<(), String> {
        self.swarm
            .behaviour_mut()
            .reqresp
            .send_response(channel, response)
            .map_err(|_| "Failed to send response: channel closed".to_string())
    }

    /// Subscribe to a gossip topic.
    pub fn subscribe(&mut self, topic: &IdentTopic) -> Result<(), String> {
        self.swarm
            .behaviour_mut()
            .gossipsub
            .subscribe(topic)
            .map_err(|e| format!("Failed to subscribe to topic: {e:?}"))
            .map(|_| ())
    }

    /// Publish a message to a gossip topic.
    pub fn publish(&mut self, topic: IdentTopic, data: Vec<u8>) -> Result<(), String> {
        self.swarm
            .behaviour_mut()
            .gossipsub
            .publish(topic, data)
            .map_err(|e| format!("Failed to publish message: {e:?}"))
            .map(|_| ())
    }

    /// Poll swarm events for a given duration, auto-responding to incoming requests
    /// and processing gossipsub events so the mesh can form.
    pub async fn process_events_for(&mut self, duration: Duration) {
        let deadline = tokio::time::Instant::now() + duration;
        loop {
            let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
            if remaining.is_zero() {
                break;
            }
            match tokio::time::timeout(remaining, self.swarm.select_next_some()).await {
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Reqresp(ReqRespEvent::Message {
                    peer: _,
                    message:
                        ReqRespMessage::Request {
                            request_id: _,
                            request,
                            channel,
                        },
                    ..
                }))) => {
                    // Try to decode as Status and echo it back.
                    if let Ok(decompressed) = decode_request(&request) {
                        if let Ok(status) = Status::from_ssz_bytes(&decompressed) {
                            let _ = self.send_response(
                                channel,
                                vec![(RESPONSE_CODE_SUCCESS, status.as_ssz_bytes())],
                            );
                            continue;
                        }
                        // Try to decode as BlocksByRoot and respond with resource unavailable.
                        if BlocksByRootV1Request::from_ssz_bytes(&decompressed).is_ok() {
                            let _ = self.send_response(
                                channel,
                                vec![(RESPONSE_CODE_RESOURCE_UNAVAILABLE, vec![])],
                            );
                            continue;
                        }
                    }
                    let _ = self
                        .send_response(channel, vec![(RESPONSE_CODE_RESOURCE_UNAVAILABLE, vec![])]);
                }
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(
                    GossipsubEvent::Message {
                        propagation_source,
                        message,
                        ..
                    },
                ))) => {
                    let topic_str = message.topic.into_string();
                    let topic = IdentTopic::new(topic_str);
                    self.gossip_messages
                        .push((propagation_source, topic, message.data));
                }
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(_))) => {}
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Identify(_))) => {}
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Ping(_))) => {}
                Ok(_) => {}
                Err(_) => break,
            }
        }
    }

    /// Wait for the swarm to report its listen address.
    pub async fn get_listen_address(&mut self) -> Option<Multiaddr> {
        let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
        loop {
            let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
            if remaining.is_zero() {
                return None;
            }
            match timeout(remaining, self.swarm.select_next_some()).await {
                Ok(SwarmEvent::NewListenAddr { address, .. }) => return Some(address),
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(
                    GossipsubEvent::Message {
                        propagation_source,
                        message,
                        ..
                    },
                ))) => {
                    let topic_str = message.topic.into_string();
                    let topic = IdentTopic::new(topic_str);
                    self.gossip_messages
                        .push((propagation_source, topic, message.data));
                    continue;
                }
                Ok(_) => continue,
                Err(_) => return None,
            }
        }
    }

    /// Drain all received gossip messages.
    pub fn drain_gossip_messages(&mut self) -> Vec<(PeerId, IdentTopic, Vec<u8>)> {
        std::mem::take(&mut self.gossip_messages)
    }

    /// Wait for a gossip message and return it.
    pub async fn wait_for_gossip_message(&mut self) -> Option<(PeerId, IdentTopic, Vec<u8>)> {
        loop {
            match self.swarm.next().await {
                Some(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(
                    GossipsubEvent::Message {
                        propagation_source,
                        message,
                        ..
                    },
                ))) => {
                    let topic_str = message.topic.into_string();
                    let topic = IdentTopic::new(topic_str);
                    return Some((propagation_source, topic, message.data));
                }
                Some(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(_))) => continue,
                Some(SwarmEvent::Behaviour(MockBehaviourEvent::Reqresp(
                    ReqRespEvent::Message { .. },
                ))) => continue,
                Some(SwarmEvent::Behaviour(MockBehaviourEvent::Reqresp(
                    ReqRespEvent::OutboundFailure { .. },
                ))) => continue,
                Some(SwarmEvent::NewListenAddr { .. }) => continue,
                Some(_) => continue,
                None => return None,
            }
        }
    }
}
