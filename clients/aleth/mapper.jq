# Removes all empty keys and values in input.
def remove_empty:
  . | walk(
    if type == "object" then
      with_entries(
        select(
          .value != null and
          .value != "" and
          .key != null and
          .key != ""
        )
      )
    else .
    end
  )
;

# Converts number to hex, from https://rosettacode.org/wiki/Non-decimal_radices/Convert#jq
def int_to_hex:
  def stream:
    recurse(if . > 0 then ./16|floor else empty end) | . % 16 ;
  if . == 0 then "0x0"
  else "0x" + ([stream] | reverse | .[1:] | map(if .<10 then 48+. else 87+. end) | implode)
  end
;

# Converts decimal number in string to hex.
def to_hex:
  if . != null and startswith("0x") then . else
    if (. != null and . != "") then .|tonumber|int_to_hex else . end
  end
;

# Zero-pads hex string.
def infix_zeros_to_length(s;l):
  if . != null then
    (.[0:s])+("0"*(l-(.|length)))+(.[s:l])
  else .
  end
;

# Removes 0x prefix from object keys.
def un0x_keys:
  with_entries({key: .key|ltrimstr("0x"), value})
;

# This returns the known precompiles.
#
# Aleth doesn't like it when precompiles have code/storage/nonce
# so these fields need to be set to the empty string in order to
# override any such fields in the input genesis. The empty fields
# are then dropped in the final remove_empty pass.
def precompiles:
  {
    "0000000000000000000000000000000000000001": {
      "precompiled": {"name": "ecrecover", "linear": {"base": 3000, "word": 0}},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000002": {
      "precompiled": {"name": "sha256", "linear": {"base": 60, "word": 12}},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000003": {
      "precompiled": {"name": "ripemd160", "linear": {"base": 600, "word": 120}},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000004": {
      "precompiled": {"name": "identity", "linear": {"base": 15, "word": 3}},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000005": {
      "precompiled": {"name": "modexp"},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000006": {
      "precompiled": {"name": "alt_bn128_G1_add", "linear": {"base": 500, "word": 0}},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000007": {
      "precompiled": {"name": "alt_bn128_G1_mul", "linear": {"base": 40000, "word": 0}},
      "code": "",
      "storage": "",
      "nonce": "",
    },
    "0000000000000000000000000000000000000008": {
      "precompiled": {"name": "alt_bn128_pairing_product"},
      "code": "",
      "storage": "",
      "nonce": "",
    },
  }
;

{
  "sealEngine": (if env.HIVE_SKIP_POW then "NoProof" else "Ethash" end),
  "params": {
    # Fork configuration
    "homesteadForkBlock": env.HIVE_FORK_HOMESTEAD|to_hex,
    "daoHardforkBlock": env.HIVE_FORK_DAO_BLOCK|to_hex,
    "EIP150ForkBlock": env.HIVE_FORK_TANGERINE|to_hex,
    "EIP158ForkBlock": env.HIVE_FORK_SPURIOUS|to_hex,
    "byzantiumForkBlock": env.HIVE_FORK_SPURIOUS|to_hex,
    "constantinopleForkBlock": env.HIVE_FORK_CONSTANTINOPLE|to_hex,
    "constantinopleFixForkBlock": env.HIVE_FORK_PETERSBURG|to_hex,
    "istanbulForkBlock": env.HIVE_FORK_ISTANBUL|to_hex,
    "muirGlacierForkBlock": env.HIVE_FORK_MUIR_GLACIER|to_hex,

    # Other chain parameters
    "maximumExtraDataSize": "0x20",
    "tieBreakingGas": false,
    "minGasLimit": "0x0",
    "gasLimitBoundDivisor": "0x400",
    "minimumDifficulty": "0x20000",
    "difficultyBoundDivisor": "0x800",
    "durationLimit": "0x0d",
    "blockReward": "0x4563918244F40000",
    "registrar": "0x0000000000000000000000000000000000000000",
    "networkID": env.HIVE_NETWORK_ID|to_hex,
    "chainID": env.HIVE_CHAIN_ID|to_hex,
  },
  "genesis": {
    "nonce": .nonce|infix_zeros_to_length(2;18),
    "mixHash": .mixHash,
    "difficulty": .difficulty,
    "author": .coinbase,
    "timestamp": .timestamp,
    "parentHash": .parentHash,
    "extraData": .extraData,
    "gasLimit": .gasLimit,
  },
  "accounts": ((.alloc|un0x_keys) * precompiles),
}|remove_empty
