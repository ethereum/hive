//! Client-neutral adapter for the Hive-only Lean proposal-signer operation.
//!
//! Used by the signer lifecycle suite; cryptographic checks run independently.

#![allow(dead_code)]

use alloy_primitives::{hex, B256};
use anyhow::{bail, Context};
use hivesim::Client;
use reqwest::Client as HttpClient;
use serde::{Deserialize, Serialize};

use crate::utils::util::{http_client, lean_api_url};

pub(crate) const SIGN_PROPOSAL_PATH: &str = "/lean/v0/test_driver/signer/sign_proposal";
const LEAN_PUBLIC_KEY_BYTES: usize = 52;

/// Request envelope shared by Hive and Lean client signer adapters.
#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SignProposalRequest {
    pub(crate) request_id: String,
    pub(crate) validator_index: u64,
    pub(crate) slot: u64,
    pub(crate) block_root: B256,
}

/// Client-observable outcome. Refusals are expected safety decisions; errors
/// mean the client could not complete the signing operation.
#[derive(Clone, Copy, Debug, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub(crate) enum SignProposalStatus {
    Signed,
    Refused,
    Error,
}

/// Response envelope returned by a Lean client signer adapter.
#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SignProposalResponse {
    pub(crate) request_id: String,
    pub(crate) status: SignProposalStatus,
    pub(crate) validator_index: u64,
    pub(crate) slot: u64,
    pub(crate) block_root: B256,
    pub(crate) proposal_public_key: Option<String>,
    pub(crate) signature: Option<String>,
    pub(crate) error_code: Option<String>,
    pub(crate) error_message: Option<String>,
}

/// HTTP adapter for the client-neutral signer operation. It validates the
/// response envelope, while cryptographic verification remains a scenario
/// assertion so it is independent of the client's transport implementation.
#[derive(Clone, Debug)]
pub(crate) struct SignerOperationClient {
    http: HttpClient,
    endpoint: String,
}

impl SignerOperationClient {
    pub(crate) fn for_hive_client(client: &Client) -> Self {
        Self::new(http_client(), lean_api_url(client, SIGN_PROPOSAL_PATH))
    }

    pub(crate) fn new(http: HttpClient, endpoint: impl Into<String>) -> Self {
        Self {
            http,
            endpoint: endpoint.into(),
        }
    }

    pub(crate) async fn sign_proposal(
        &self,
        request: &SignProposalRequest,
    ) -> anyhow::Result<SignProposalResponse> {
        let response = self
            .http
            .post(&self.endpoint)
            .json(request)
            .send()
            .await
            .with_context(|| {
                format!("failed to call Lean signer operation at {}", self.endpoint)
            })?;
        let status = response.status();
        let body = response
            .bytes()
            .await
            .context("failed to read Lean signer response body")?;

        if !status.is_success() {
            bail!(
                "Lean signer operation returned HTTP {status}: {}",
                response_body_summary(&body)
            );
        }

        let response: SignProposalResponse = serde_json::from_slice(&body)
            .context("failed to decode Lean signer response envelope")?;
        validate_response(request, &response)?;
        Ok(response)
    }
}

fn validate_response(
    request: &SignProposalRequest,
    response: &SignProposalResponse,
) -> anyhow::Result<()> {
    if response.request_id != request.request_id {
        bail!(
            "Lean signer response requestId mismatch: expected {:?}, got {:?}",
            request.request_id,
            response.request_id
        );
    }
    if response.validator_index != request.validator_index {
        bail!(
            "Lean signer response validatorIndex mismatch: expected {}, got {}",
            request.validator_index,
            response.validator_index
        );
    }
    if response.slot != request.slot {
        bail!(
            "Lean signer response slot mismatch: expected {}, got {}",
            request.slot,
            response.slot
        );
    }
    if response.block_root != request.block_root {
        bail!(
            "Lean signer response blockRoot mismatch: expected {:#x}, got {:#x}",
            request.block_root,
            response.block_root
        );
    }

    match response.status {
        SignProposalStatus::Signed => {
            let public_key = response
                .proposal_public_key
                .as_deref()
                .context("signed response is missing proposalPublicKey")?;
            validate_hex("proposalPublicKey", public_key, Some(LEAN_PUBLIC_KEY_BYTES))?;
            let signature = response
                .signature
                .as_deref()
                .context("signed response is missing signature")?;
            validate_hex("signature", signature, None)?;
            if response.error_code.is_some() {
                bail!("signed response must not contain errorCode");
            }
            if response.error_message.is_some() {
                bail!("signed response must not contain errorMessage");
            }
        }
        SignProposalStatus::Refused | SignProposalStatus::Error => {
            if response.signature.is_some() {
                bail!("refused/error response must not contain a signature");
            }
            if response
                .error_code
                .as_deref()
                .is_none_or(|code| code.is_empty())
            {
                bail!("refused/error response is missing a stable errorCode");
            }
        }
    }

    Ok(())
}

fn validate_hex(field: &str, value: &str, expected_bytes: Option<usize>) -> anyhow::Result<()> {
    let encoded = value
        .strip_prefix("0x")
        .with_context(|| format!("{field} must be 0x-prefixed"))?;
    if encoded.is_empty() {
        bail!("{field} must not be empty");
    }
    let decoded = hex::decode(encoded).with_context(|| format!("{field} is not valid hex"))?;
    if let Some(expected_bytes) = expected_bytes {
        if decoded.len() != expected_bytes {
            bail!(
                "{field} must contain {expected_bytes} bytes, got {}",
                decoded.len()
            );
        }
    }
    Ok(())
}

fn response_body_summary(body: &[u8]) -> String {
    const MAX_SUMMARY_BYTES: usize = 512;
    let summary = &body[..body.len().min(MAX_SUMMARY_BYTES)];
    let mut summary = String::from_utf8_lossy(summary).into_owned();
    if body.len() > MAX_SUMMARY_BYTES {
        summary.push_str("...");
    }
    summary
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use reqwest::StatusCode;
    use serde_json::{json, Value};
    use tokio::{
        io::{AsyncReadExt, AsyncWriteExt},
        net::TcpListener,
        sync::Mutex,
        task::JoinHandle,
    };

    use super::*;

    const BLOCK_ROOT: B256 = B256::repeat_byte(0x42);

    fn request() -> SignProposalRequest {
        SignProposalRequest {
            request_id: "request-1".to_string(),
            validator_index: 7,
            slot: 3,
            block_root: BLOCK_ROOT,
        }
    }

    async fn spawn_json_server(
        status: StatusCode,
        response: Value,
    ) -> (String, Arc<Mutex<Option<Value>>>, JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0")
            .await
            .expect("test server should bind");
        let address = listener
            .local_addr()
            .expect("test server should have an address");
        let captured = Arc::new(Mutex::new(None));
        let captured_for_server = Arc::clone(&captured);
        let task = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.expect("test server should accept");
            let request_bytes = read_http_request(&mut stream).await;
            let separator = request_bytes
                .windows(4)
                .position(|window| window == b"\r\n\r\n")
                .expect("request should contain a header separator");
            let headers = String::from_utf8_lossy(&request_bytes[..separator]);
            assert!(
                headers.starts_with("POST /lean/v0/test_driver/signer/sign_proposal HTTP/1.1"),
                "unexpected request line: {headers}"
            );
            let body: Value = serde_json::from_slice(&request_bytes[separator + 4..])
                .expect("request body should be JSON");
            *captured_for_server.lock().await = Some(body);

            let body = serde_json::to_vec(&response).expect("response should serialize");
            let response_headers = format!(
                "HTTP/1.1 {} {}\r\ncontent-type: application/json\r\ncontent-length: {}\r\nconnection: close\r\n\r\n",
                status.as_u16(),
                status.canonical_reason().unwrap_or("Test"),
                body.len()
            );
            stream
                .write_all(response_headers.as_bytes())
                .await
                .expect("test response headers should write");
            stream
                .write_all(&body)
                .await
                .expect("test response body should write");
        });

        (
            format!("http://{address}{SIGN_PROPOSAL_PATH}"),
            captured,
            task,
        )
    }

    async fn read_http_request(stream: &mut tokio::net::TcpStream) -> Vec<u8> {
        let mut bytes = Vec::new();
        let mut buffer = [0_u8; 4096];
        loop {
            let count = stream
                .read(&mut buffer)
                .await
                .expect("test request should read");
            assert!(count > 0, "connection closed before request completed");
            bytes.extend_from_slice(&buffer[..count]);

            let Some(separator) = bytes.windows(4).position(|window| window == b"\r\n\r\n") else {
                continue;
            };
            let headers = String::from_utf8_lossy(&bytes[..separator]);
            let content_length = headers
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())
                        .flatten()
                })
                .expect("request should contain content-length");
            if bytes.len() >= separator + 4 + content_length {
                bytes.truncate(separator + 4 + content_length);
                return bytes;
            }
        }
    }

    #[tokio::test]
    async fn sends_the_common_request_and_accepts_a_signed_response() {
        let public_key = format!("0x{}", "11".repeat(LEAN_PUBLIC_KEY_BYTES));
        let response = json!({
            "requestId": "request-1",
            "status": "signed",
            "validatorIndex": 7,
            "slot": 3,
            "blockRoot": format!("{BLOCK_ROOT:#x}"),
            "proposalPublicKey": public_key,
            "signature": "0x0102",
            "errorCode": null,
            "errorMessage": null
        });
        let (endpoint, captured, server) = spawn_json_server(StatusCode::OK, response).await;
        let adapter = SignerOperationClient::new(HttpClient::new(), endpoint);

        let signed = adapter
            .sign_proposal(&request())
            .await
            .expect("signed response should pass the adapter contract");
        server.await.expect("test server should finish");

        assert_eq!(signed.status, SignProposalStatus::Signed);
        assert_eq!(signed.signature.as_deref(), Some("0x0102"));
        assert_eq!(
            *captured.lock().await,
            Some(json!({
                "requestId": "request-1",
                "validatorIndex": 7,
                "slot": 3,
                "blockRoot": format!("{BLOCK_ROOT:#x}")
            }))
        );
    }

    #[tokio::test]
    async fn accepts_a_structured_refusal_without_treating_it_as_transport_failure() {
        let response = json!({
            "requestId": "request-1",
            "status": "refused",
            "validatorIndex": 7,
            "slot": 3,
            "blockRoot": format!("{BLOCK_ROOT:#x}"),
            "proposalPublicKey": null,
            "signature": null,
            "errorCode": "slot_already_consumed",
            "errorMessage": "proposal slot already signed"
        });
        let (endpoint, _, server) = spawn_json_server(StatusCode::OK, response).await;
        let adapter = SignerOperationClient::new(HttpClient::new(), endpoint);

        let refused = adapter
            .sign_proposal(&request())
            .await
            .expect("structured refusal should be an observable signer outcome");
        server.await.expect("test server should finish");

        assert_eq!(refused.status, SignProposalStatus::Refused);
        assert_eq!(refused.error_code.as_deref(), Some("slot_already_consumed"));
        assert!(refused.signature.is_none());
    }

    #[tokio::test]
    async fn rejects_a_response_that_does_not_echo_the_requested_signing_context() {
        let response = json!({
            "requestId": "another-request",
            "status": "signed",
            "validatorIndex": 7,
            "slot": 3,
            "blockRoot": format!("{BLOCK_ROOT:#x}"),
            "proposalPublicKey": format!("0x{}", "11".repeat(LEAN_PUBLIC_KEY_BYTES)),
            "signature": "0x0102",
            "errorCode": null,
            "errorMessage": null
        });
        let (endpoint, _, server) = spawn_json_server(StatusCode::OK, response).await;
        let adapter = SignerOperationClient::new(HttpClient::new(), endpoint);

        let error = adapter
            .sign_proposal(&request())
            .await
            .expect_err("mismatched response must be rejected");
        server.await.expect("test server should finish");

        assert!(error.to_string().contains("requestId mismatch"));
    }

    #[tokio::test]
    async fn rejects_a_signed_response_without_signature_material() {
        let response = json!({
            "requestId": "request-1",
            "status": "signed",
            "validatorIndex": 7,
            "slot": 3,
            "blockRoot": format!("{BLOCK_ROOT:#x}"),
            "proposalPublicKey": format!("0x{}", "11".repeat(LEAN_PUBLIC_KEY_BYTES)),
            "signature": null,
            "errorCode": null,
            "errorMessage": null
        });
        let (endpoint, _, server) = spawn_json_server(StatusCode::OK, response).await;
        let adapter = SignerOperationClient::new(HttpClient::new(), endpoint);

        let error = adapter
            .sign_proposal(&request())
            .await
            .expect_err("missing signature must be rejected");
        server.await.expect("test server should finish");

        assert!(error.to_string().contains("missing signature"));
    }
}
