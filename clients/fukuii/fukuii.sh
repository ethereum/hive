#!/bin/bash
# Hive entry point for Fukuii
# Uses the "hive" network config (vanilla Ethereum, no ETC baggage).
# Translates HIVE_* env vars to -Dfukuii.blockchains.hive.* overrides.
set -e

DATADIR="/app/data"
GENESIS_FILE="/genesis.json"
JWT_SECRET_FILE="/jwtsecret"
CONFIG_DIR="/app/hive-conf"

mkdir -p "$DATADIR" "$CONFIG_DIR"

# ==============================================================================
# Fork configuration from HIVE_FORK_* environment variables
# ==============================================================================

MAX="1000000000000000000"
HOMESTEAD=${HIVE_FORK_HOMESTEAD:-$MAX}
TANGERINE=${HIVE_FORK_TANGERINE:-$MAX}
SPURIOUS=${HIVE_FORK_SPURIOUS:-$MAX}
BYZANTIUM=${HIVE_FORK_BYZANTIUM:-$MAX}
CONSTANTINOPLE=${HIVE_FORK_CONSTANTINOPLE:-$MAX}
PETERSBURG=${HIVE_FORK_PETERSBURG:-$MAX}
ISTANBUL=${HIVE_FORK_ISTANBUL:-$MAX}
# HIVE_FORK_MUIR_GLACIER is underscored. That is the name execution-apis'
# tests/forkenv.json and hive's own clients/go-ethereum/mapper.jq export; the
# un-underscored HIVE_FORK_MUIRGLACIER survives only in a stale comment in hive's
# geth.sh and is never set by anything. Reading the wrong one left Muir Glacier at the
# $MAX sentinel, so it never entered the EIP-2124 checksum chain. Measured on
# rpc-compat: fukuii advertised 0xe54f18d6 — exactly the checksum of the fixture's fork
# list with block 21 removed — where peers and eth_config expect 0xe272ecbe.
MUIRGLACIER=${HIVE_FORK_MUIR_GLACIER:-$MAX}
BERLIN=${HIVE_FORK_BERLIN:-$MAX}
LONDON=${HIVE_FORK_LONDON:-$MAX}

NETWORK_ID=${HIVE_NETWORK_ID:-1}
CHAIN_ID=${HIVE_CHAIN_ID:-1}
TTD=${HIVE_TERMINAL_TOTAL_DIFFICULTY:-$MAX}
SHANGHAI_TS=${HIVE_SHANGHAI_TIMESTAMP:-}
CANCUN_TS=${HIVE_CANCUN_TIMESTAMP:-}
PRAGUE_TS=${HIVE_PRAGUE_TIMESTAMP:-}
OSAKA_TS=${HIVE_OSAKA_TIMESTAMP:-}
# BPO1/BPO2 (EIP-7892 blob-parameter-only forks). These carry no EVM changes, but
# they ARE forks: they must enter the EIP-6122 fork-id checksum chain, and they
# select the blob target/max and base-fee update fraction. Omitting them made the
# node advertise a fork id computed over [cancun, prague, osaka] only, which peers
# reject with "wrong fork ID in status".
BPO1_TS=${HIVE_BPO1_TIMESTAMP:-}
BPO2_TS=${HIVE_BPO2_TIMESTAMP:-}
AMSTERDAM_TS=${HIVE_AMSTERDAM_TIMESTAMP:-}

# go-ethereum's --miner.gaslimit. hive's rpc-compat simulator sets this explicitly
# ("Match execution-apis' Geth default so all clients build the same next-block gas
# limit") and the execution-apis testing_* fixtures were recorded against it: the
# proposer steps the gas limit toward this target by parent/1024-1 per block, so a
# mismatch changes every built block's hash. Fukuii's own default is already 60M.
TARGET_GAS_LIMIT=${HIVE_TARGET_GAS_LIMIT:-}

# ==============================================================================
# Genesis: convert geth format to Fukuii format
# ==============================================================================

if [ -f "$GENESIS_FILE" ]; then
    jq -f /mapper.jq "$GENESIS_FILE" > "$CONFIG_DIR/genesis.json"
    # EIP-6110 deposit contract. Genesis-declared and NOT a universal constant — hive's
    # rpc-compat fixture declares the zero address while mainnet uses 0x00000000219ab540...
    # eth_config (EIP-7910) must advertise the chain's own value, so read it from the
    # genesis rather than falling back to the mainnet contract.
    DEPOSIT_CONTRACT=$(jq -r '.config.depositContractAddress // empty' "$GENESIS_FILE" 2>/dev/null || true)

    # Arrow Glacier (EIP-4345), Gray Glacier (EIP-5133) and the post-Merge net-split block
    # (EIP-3675). These have no EVM semantics, but go-ethereum's `gatherForks` enumerates
    # every *Block field of ChainConfig, so all three enter the EIP-2124 fork-id checksum
    # chain. hive exports HIVE_FORK_* only for the forks it models, and these three are not
    # among them, so read them from the genesis the simulator actually handed us.
    #
    # Omitting them is not cosmetic: on this fixture the correct checksum is 0xe272ecbe and
    # the checksum without them is 0x5e0cb820, which every geth peer rejects with
    # "wrong fork ID in status" — i.e. no peering at all.
    ARROW_GLACIER=$(jq -r '.config.arrowGlacierBlock // empty' "$GENESIS_FILE" 2>/dev/null || true)
    GRAY_GLACIER=$(jq -r '.config.grayGlacierBlock // empty' "$GENESIS_FILE" 2>/dev/null || true)
    MERGE_NETSPLIT=$(jq -r '.config.mergeNetsplitBlock // empty' "$GENESIS_FILE" 2>/dev/null || true)
fi

# hive does export all four of these, under its own names, and a simulator may hand us a
# genesis whose `config` block omits them while the environment carries them. Env wins
# where present; the genesis reads above remain the fallback. Both real fixtures
# (execution-apis tests/forkenv.json and devp2p's testdata) agree on every value, so this
# changes nothing there and only removes the dependency on the genesis carrying them.
ARROW_GLACIER=${HIVE_FORK_ARROW_GLACIER:-$ARROW_GLACIER}
GRAY_GLACIER=${HIVE_FORK_GRAY_GLACIER:-$GRAY_GLACIER}
MERGE_NETSPLIT=${HIVE_MERGE_BLOCK_ID:-$MERGE_NETSPLIT}
DEPOSIT_CONTRACT=${HIVE_DEPOSIT_CONTRACT_ADDRESS:-$DEPOSIT_CONTRACT}

# ==============================================================================
# JWT secret for Engine API
# ==============================================================================

echo "0x7365637265747365637265747365637265747365637265747365637265747365" > "$JWT_SECRET_FILE"

# ==============================================================================
# Build JVM flags — "hive" network with clean Ethereum defaults
# ==============================================================================

FLAGS=""
FLAGS="$FLAGS -Dfukuii.datadir=$DATADIR"
FLAGS="$FLAGS -Dfukuii.blockchains.network=hive"
# The port hive probes for readiness (libhive default 8545; the ethereum/sync
# sink node is probed on 8551). StdNode binds this port last. See Dockerfile.
FLAGS="$FLAGS -Dfukuii.network.readiness-port=${HIVE_CHECK_LIVE_PORT:-8545}"

# Unprotected (pre-EIP-155) transactions over RPC are refused by default, as in
# go-ethereum; the graphql simulator sets HIVE_ALLOW_UNPROTECTED_TX because its
# sendRawTransaction vectors are unprotected (geth.sh maps it the same way).
if [ -n "$HIVE_ALLOW_UNPROTECTED_TX" ]; then
    FLAGS="$FLAGS -Dfukuii.network.rpc.allow-unprotected-txs=true"
fi

# Chain/network identity
FLAGS="$FLAGS -Dfukuii.blockchains.hive.chain-id=$CHAIN_ID"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.network-id=$NETWORK_ID"

# Genesis override
if [ -f "$CONFIG_DIR/genesis.json" ]; then
    FLAGS="$FLAGS -Dfukuii.blockchains.hive.custom-genesis-file=$CONFIG_DIR/genesis.json"
fi

# Standard Ethereum fork overrides
FLAGS="$FLAGS -Dfukuii.blockchains.hive.homestead-block-number=$HOMESTEAD"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.eip150-block-number=$TANGERINE"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.eip155-block-number=$SPURIOUS"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.eip160-block-number=$SPURIOUS"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.eip161-block-number=$SPURIOUS"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.byzantium-block-number=$BYZANTIUM"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.constantinople-block-number=$CONSTANTINOPLE"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.petersburg-block-number=$PETERSBURG"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.istanbul-block-number=$ISTANBUL"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.muir-glacier-block-number=$MUIRGLACIER"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.berlin-block-number=$BERLIN"
FLAGS="$FLAGS -Dfukuii.blockchains.hive.olympia-block-number=$LONDON"

# Terminal total difficulty: only set when the hive sim explicitly provides one.
# fukuii's SNAPSyncController treats `terminal-total-difficulty.isDefined` as
# "post-merge chain" and then waits for engine_forkchoiceUpdated from the CL
# before picking a SNAP pivot (engine-api-required = true is the sync.conf
# default — see #1208). Pre-merge hive sims (including ethereum/sync) have no
# CL, so passing the $MAX sentinel here would wedge the sink at head=0 for the
# full 60s simulator budget and fail every fukuii-tagged sync sub-test.
if [ "$TTD" != "$MAX" ]; then
    FLAGS="$FLAGS -Dfukuii.blockchains.hive.terminal-total-difficulty=$TTD"
fi

# Timestamp-based forks
[ -n "$SHANGHAI_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.shanghai-timestamp=$SHANGHAI_TS"
[ -n "$CANCUN_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.cancun-timestamp=$CANCUN_TS"
[ -n "$PRAGUE_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.prague-timestamp=$PRAGUE_TS"
[ -n "$OSAKA_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.osaka-timestamp=$OSAKA_TS"
[ -n "$BPO1_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.bpo1-timestamp=$BPO1_TS"
[ -n "$BPO2_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.bpo2-timestamp=$BPO2_TS"
[ -n "$AMSTERDAM_TS" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.amsterdam-timestamp=$AMSTERDAM_TS"

# EIP-7840 blob schedule (#1637): target/max blobs per block and the blob base-fee update fraction per
# fork. hive exports these next to the timestamps (forkenv.json); hive-chain.conf carries the same
# defaults go-ethereum's mapper.jq falls back to. HIVE_OSAKA_BLOB_* is deliberately NOT mapped:
# go-ethereum master has no osaka entry (Osaka inherits Prague's), so it ignores those values, and
# fukuii's loader refuses an osaka entry that differs from Prague's - mapping it could only turn a
# hive-side mismatch into a client that does not start. Names are spelled out (no loop) so
# HiveAdapterForkEnvSpec can check each against what hive exports.
blob_param() { # $1 = fork, $2 = key, $3 = value
    [ -n "$3" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.blob-schedule.$1.$2=$3"
    return 0
}
blob_param cancun target "${HIVE_CANCUN_BLOB_TARGET:-}"
blob_param cancun max "${HIVE_CANCUN_BLOB_MAX:-}"
blob_param cancun base-fee-update-fraction "${HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION:-}"
blob_param prague target "${HIVE_PRAGUE_BLOB_TARGET:-}"
blob_param prague max "${HIVE_PRAGUE_BLOB_MAX:-}"
blob_param prague base-fee-update-fraction "${HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION:-}"
blob_param bpo1 target "${HIVE_BPO1_BLOB_TARGET:-}"
blob_param bpo1 max "${HIVE_BPO1_BLOB_MAX:-}"
blob_param bpo1 base-fee-update-fraction "${HIVE_BPO1_BLOB_BASE_FEE_UPDATE_FRACTION:-}"
blob_param bpo2 target "${HIVE_BPO2_BLOB_TARGET:-}"
blob_param bpo2 max "${HIVE_BPO2_BLOB_MAX:-}"
blob_param bpo2 base-fee-update-fraction "${HIVE_BPO2_BLOB_BASE_FEE_UPDATE_FRACTION:-}"

# EIP-6110 deposit contract address, as declared by the fixture genesis (see above).
[ -n "$DEPOSIT_CONTRACT" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.deposit-contract-address=$DEPOSIT_CONTRACT"

# Fork-id-only block forks read from the genesis above (see the comment there).
[ -n "$ARROW_GLACIER" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.arrow-glacier-block-number=$ARROW_GLACIER"
[ -n "$GRAY_GLACIER" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.gray-glacier-block-number=$GRAY_GLACIER"
[ -n "$MERGE_NETSPLIT" ] && FLAGS="$FLAGS -Dfukuii.blockchains.hive.merge-netsplit-block-number=$MERGE_NETSPLIT"

# DAO hard fork. hive's consensus simulator sets HIVE_FORK_DAO_VOTE=1 on every test and
# HIVE_FORK_DAO_BLOCK to the fork's own value (e.g. 5 for HomesteadToDaoAt5, 2000 where the
# fork is out of reach); go-ethereum's mapper maps them to daoForkBlock / daoForkSupport=true.
# hive-chain.conf has `dao = null`, so without this block the DAO fork did not exist here:
# extraData in [fork, fork+10) was never checked (bcHomesteadToDao UncleExtradata) and the
# 116 drain-list balances were never moved to the refund contract at the fork block (state
# mismatch in DaoTransactions*). Only when BOTH are set, like geth's DAOForkSupport: a
# simulator that sets a DAO block without the vote keeps `dao = null` and an unchanged
# fork id.
#
# Every key DaoForkConfig reads is passed, because BlockchainConfig wraps it in
# Try(...).toOption: a missing required key (fork-block-number, fork-block-hash,
# include-on-fork-id-list) silently yields NO DAO fork rather than an error.
#
# fork-block-hash is consumed ONLY by the peer-handshake ForkResolver (it accepts a peer iff
# its header at the fork block has this hash). The consensus sim has no peers, so a
# well-formed all-zero hash is used; the mainnet hash would be equally meaningless here.
#
# The drain list is passed as numerically-indexed system properties
# (...drain-list.0=, .1=, ...). Typesafe Config turns an object whose keys are all
# non-negative integers into a list when a list is requested (HOCON spec, "numerically-indexed
# objects"; DefaultTransformer), which is what getStringList does. Each entry is checked to
# be exactly 40 hex chars, so nothing shell-special can reach $FLAGS.
# HiveDaoForkConfigSpec loads properties of exactly this form and asserts 116 addresses.
DAO_BLOCK=${HIVE_FORK_DAO_BLOCK:-}
case "${HIVE_FORK_DAO_VOTE:-}" in 1|true|True|TRUE) DAO_VOTE=1 ;; *) DAO_VOTE= ;; esac
if [ -n "$DAO_BLOCK" ] && [ -n "$DAO_VOTE" ]; then
    DAO_PREFIX="-Dfukuii.blockchains.hive.dao"
    FLAGS="$FLAGS $DAO_PREFIX.fork-block-number=$DAO_BLOCK"
    FLAGS="$FLAGS $DAO_PREFIX.fork-block-hash=0000000000000000000000000000000000000000000000000000000000000000"
    FLAGS="$FLAGS $DAO_PREFIX.block-extra-data=dao-hard-fork"
    FLAGS="$FLAGS $DAO_PREFIX.block-extra-data-range=10"
    FLAGS="$FLAGS $DAO_PREFIX.refund-contract-address=bf4ed7b27f1d666546e30d74d50d173d20bca754"
    FLAGS="$FLAGS $DAO_PREFIX.include-on-fork-id-list=true"
    DAO_I=0
    while IFS= read -r DAO_ADDR; do
        [ -z "$DAO_ADDR" ] && continue
        if ! printf '%s' "$DAO_ADDR" | grep -Eq '^[0-9a-f]{40}$'; then
            echo "dao-drain-list.txt: malformed address '$DAO_ADDR'" >&2
            exit 1
        fi
        FLAGS="$FLAGS $DAO_PREFIX.drain-list.$DAO_I=$DAO_ADDR"
        DAO_I=$((DAO_I + 1))
    done < /dao-drain-list.txt
    if [ "$DAO_I" -ne 116 ]; then
        echo "dao-drain-list.txt: expected 116 addresses, found $DAO_I" >&2
        exit 1
    fi
fi

# RPC
[ -n "$TARGET_GAS_LIMIT" ] && FLAGS="$FLAGS -Dfukuii.mining.gas-limit-target=$TARGET_GAS_LIMIT"

FLAGS="$FLAGS -Dfukuii.network.rpc.http.enabled=true"
FLAGS="$FLAGS -Dfukuii.network.rpc.http.interface=0.0.0.0"
FLAGS="$FLAGS -Dfukuii.network.rpc.http.port=8545"
# Pekko HTTP request timeout, raised from the shipped 30 s (which matches go-ethereum's
# auth-RPC write timeout) for hive only. Test vectors such as static_Call50000* and
# tstore_wide spend ~5e9 gas in one block — far beyond any mainnet block — and under
# hive's parallel clients a cold newPayload for them measured ~25 s (4 contending JVMs)
# to ~52 s (8). At 30 s Pekko answers 503 while execution continues. The Engine API
# spec sets no server-side limit (the consensus client MAY wait longer than its 8 s).
# Applies to both the JSON-RPC and Engine API servers (system properties override
# engine-api-system.conf). idle-timeout must stay above request-timeout or the
# connection is closed under an in-flight request first.
FLAGS="$FLAGS -Dpekko.http.server.request-timeout=120s"
FLAGS="$FLAGS -Dpekko.http.server.idle-timeout=180s"
# `txpool` added 2026-09-20 (#1407). The ethereum/rpc-compat suite exercises
# txpool_content, txpool_contentFrom and txpool_status; all three are
# implemented (JsonRpcController.scala:534-538, Apis.TxPool = "txpool", listed
# in NodeBuilder's `available`), but they were never served under hive because
# this line did not expose the namespace. Those three failures were a gap in
# the harness configuration, not in the client.
# `admin` added 2026-09-21 (#1407). hive/fukuii/enode.sh calls admin_nodeInfo to
# discover this node's P2P identity. admin_nodeInfo is implemented
# (JsonRpcController.scala:495, Apis.Admin = "admin") but the namespace was not
# exposed, so the call returned method-not-found, enode.sh fell through to its
# last-resort branch and emitted a literal "unknown" as the public key. hive then
# failed every devp2p test with `invalid public key (encoding/hex: invalid byte:
# U+0075 'u')` — the 'u' of "unknown". Same shape as the txpool gap above:
# implemented method, unexposed namespace, failure attributed to the client.
# `testing` added 2026-09-22. The ethereum/rpc-compat suite exercises the
# execution-apis testing_* namespace (testing_buildBlockV1, testing_commitBlockV1 —
# 9 test vectors). The namespace is implemented (jsonrpc/TestingService.scala,
# Apis.Testing = "testing") but is deliberately absent from every shipped config:
# the spec says it "MUST NOT be exposed on public-facing RPC APIs" and "is strongly
# recommended to be disabled by default". hive is exactly the environment it is for,
# so it is opted in here and ONLY here.
FLAGS="$FLAGS -Dfukuii.network.rpc.apis=eth,web3,net,debug,txpool,admin,testing"

# Engine API — only enable for post-merge chains (TTD is not MAX)
if [ "$TTD" != "$MAX" ]; then
    FLAGS="$FLAGS -Dfukuii.network.engine-api.enabled=true"
    FLAGS="$FLAGS -Dfukuii.network.engine-api.interface=0.0.0.0"
    FLAGS="$FLAGS -Dfukuii.network.engine-api.port=8551"
    FLAGS="$FLAGS -Dfukuii.network.engine-api.jwt-secret-path=$JWT_SECRET_FILE"
    FLAGS="$FLAGS -Dfukuii.mining.protocol=engine-api"
    # Hive pre-merge blocks ship with fake Ethash seals; skip PoW header check.
    FLAGS="$FLAGS -Dfukuii.mining.skip-pow-validation=true"
else
    # PoW chain — no engine API. Hive-generated chains always use fake
    # Ethash seals (HIVE_SKIP_POW honours the explicit signal; rpc-compat
    # and other sims don't set it but still feed us bogus seals), so use
    # the non-validating 'mocked' protocol unconditionally in hive mode.
    FLAGS="$FLAGS -Dfukuii.network.engine-api.enabled=false"
    FLAGS="$FLAGS -Dfukuii.mining.protocol=mocked"
fi

# P2P
FLAGS="$FLAGS -Dfukuii.network.server-address.interface=0.0.0.0"
FLAGS="$FLAGS -Dfukuii.network.server-address.port=30303"
FLAGS="$FLAGS -Dfukuii.network.discovery.interface=0.0.0.0"
FLAGS="$FLAGS -Dfukuii.network.discovery.port=30303"
# Discovery stays on. A go-ethereum sink finds its source only through discovery: hive passes
# the source's enode as --bootnodes, which seeds geth's table but is not dialed as such. geth
# dials a node only while its discovery keeps it alive and its ENR carries an `eth` entry geth
# can load and accepts. The static-nodes.json below covers fukuii sinks, not other clients.
FLAGS="$FLAGS -Dfukuii.network.discovery.discovery-enabled=true"

# Advertise the container's own address, as hive's go-ethereum adapter does with
# --nat=extip. Left to detection, the ENR carries this host's public IP, or loopback where
# detection fails; geth adopts the ENR's IP, its pings there time out, and it drops the node
# as dead before ever dialing it.
CONTAINER_IP=$(hostname -i 2>/dev/null | awk '{print $1}')
if [ -n "$CONTAINER_IP" ]; then
    FLAGS="$FLAGS -Dfukuii.network.server-address.advertised-address=$CONTAINER_IP"
    FLAGS="$FLAGS -Dfukuii.network.discovery.host=$CONTAINER_IP"
fi

# Chain import — prefer /chain.rlp, otherwise concatenate /blocks/*.rlp (consensus sim).
if [ -f "/chain.rlp" ]; then
    FLAGS="$FLAGS -Dfukuii.import-chain-file=/chain.rlp"
elif ls /blocks/*.rlp >/dev/null 2>&1; then
    cat /blocks/*.rlp > /chain.rlp
    FLAGS="$FLAGS -Dfukuii.import-chain-file=/chain.rlp"
fi

# Bootnode — write to static-nodes.json in the datadir so the node dials it directly.
# HOCON arrays can't be populated via -D system properties, so file is the reliable path.
if [ -n "$HIVE_BOOTNODE" ]; then
    echo "[\"$HIVE_BOOTNODE\"]" > "$DATADIR/static-nodes.json"
fi

# Mining
if [ -n "$HIVE_MINER" ]; then
    FLAGS="$FLAGS -Dfukuii.mining.mining-enabled=true"
    FLAGS="$FLAGS -Dfukuii.mining.coinbase=$HIVE_MINER"
fi

# The JIT is NOT capped at C1. An earlier -XX:TieredStopAtLevel=1 bought ~0.5-0.7 s of
# readiness (AOT cache on; ~1 s without) but ran EVM-heavy blocks 2-4x slower. Measured
# on 2 pinned CPUs with these flags: static_Call50000 cold newPayload 19.9 s capped vs
# 7.2 s uncapped (1 JVM), 44 s vs 25 s with 4 JVMs contending — the capped runs are what
# crossed the Engine API request timeout (HTTP 503) under hive's parallelism. A full
# 3,000-block sync-sim import is no slower uncapped (12.4 s vs 12.2 s): C2 pays for
# itself within a few thousand light blocks. The slow fukuii-sink sync runs the cap was
# meant to help stall in RegularSync for ~90 s with or without it.
#
# JVM_OPTS is shared with the AOT cache below: aot-train.sh trains through this very
# script, so the image-build training run and every hive run use one flag set by
# construction (-D properties in $FLAGS may differ; the JVM does not validate those).
JVM_OPTS=(
    -Xmx512m
    -Xms128m
    # 8M, not 2M: the EVM recurses on the JVM stack at ~3.1 KB per call level, so a
    # 1024-deep CALL/CREATE chain needs >3 MB. At 2M, chain import on `main` threw
    # StackOverflowError on 24 legacy consensus tests (Call1024*, Delegatecall1024*,
    # LoopCallsDepthThenRevert*, recursiveCreateReturnValue, CallRecursiveBombPreCall) and
    # the port never opened. Reproduced: 3M still overflows two of them; 4M and 8M pass
    # all with exact lastblockhash. Reserved, not committed, memory: untouched pages cost nothing.
    -Xss8M
    -XX:+UseG1GC
    -XX:MaxMetaspaceSize=256m
    -XX:+ExitOnOutOfMemoryError
    # JDK 25.0.4 mis-links a few i2c/c2i adapter stubs out of an AOT cache and prints
    # "Failed to link AdapterHandlerEntry ... to its code in the AOT code cache" on every
    # start, then regenerates them. Harmless but alarming in hive logs; adapters are a
    # handful of tiny stubs, and not caching them measured no slower (N=20).
    -XX:+UnlockDiagnosticVMOptions
    -XX:-AOTAdapterCaching
    # JDK-8385993: C2 ReduceAllocationMerges can silently miscompile field reads on JDK
    # 23+ (fixed in JDK 28). Also set via the image's JDK_JAVA_OPTIONS; repeated here so
    # this script is safe on any base. Needs the Unlock above. aot-train.sh boots through
    # this script, so training and every hive run see the same flags and the AOT cache's
    # flag check cannot reject the cache over this one.
    -XX:-ReduceAllocationMerges
    # Inline ProgramState's constructor wherever C2 compiles an allocation of it. The EVM
    # builds one ProgramState (a 26-field case class) per executed instruction. Compiled on
    # its own, the constructor is ~6 KB of machine code, because every reference store needs
    # a G1 barrier when the object isn't known to be fresh. That is over InlineSmallCode
    # (2500), so each allocation site called it instead of inlining it. Inlined at the
    # allocation, the barriers are elided. Measured on this image's JDK (Temurin 25.0.4),
    # legacy loopMul_d2g0v0_Cancun: 25.4 -> 18.1 CPU-s (-29%). JIT inlining only, no
    # behavioural effect. `quiet` must come first: it stops the JVM echoing the command
    # at startup.
    -XX:CompileCommand=quiet
    "-XX:CompileCommand=inline,com.chipprbots.ethereum.vm.ProgramState::<init>"
)

# ==============================================================================
# AOT cache (JDK 25, JEP 483/514) — startup time only, no behavioural effect
# ==============================================================================
# The image build (Dockerfile -> aot-train.sh) boots this node once with a hive-like
# post-merge config until the readiness port binds, stops it, and the JVM writes the
# classes it loaded and linked to $AOT_CACHE. Every hive container then maps them in
# instead of re-parsing and re-linking ~12k classes out of the 200 MB assembly.
# Measured (container start -> :8545 accepting, N=20): 5.35 s -> 3.8 s median on a
# post-merge Prague genesis, 2.7 s -> 1.4 s pre-merge. Most of what remains post-merge is
# the native KZG trusted-setup load (~1.9 s), which a class cache cannot touch.
#
# The cache holds loaded/linked class data and pre-resolved lambda call sites; the
# application's own static initialisers still run normally, so program semantics are
# unchanged. The JVM validates the cache against the JDK build, the -jar path, size and
# mtime, and the GC/heap flags; on ANY mismatch it prints an [aot] error line, ignores
# the cache (AOTMode defaults to "auto") and starts exactly as without it. A missing
# file is skipped here explicitly.
AOT_CACHE=/app/fukuii/fukuii.aot
if [ -n "$FUKUII_AOT_TRAIN" ]; then
    JVM_OPTS+=("-XX:AOTCacheOutput=$AOT_CACHE")
elif [ -f "$AOT_CACHE" ]; then
    JVM_OPTS+=("-XX:AOTCache=$AOT_CACHE")
fi

exec java \
    "${JVM_OPTS[@]}" \
    $FLAGS \
    -jar /app/fukuii/lib/fukuii-assembly.jar \
    hive
