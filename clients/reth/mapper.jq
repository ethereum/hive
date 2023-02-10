# Removes all empty keys and values in input.
def remove_empty:
  . | walk(
    if type == "object" then
      with_entries(
        select(
          .value != null and
          .value != "" and
          .value != [] and
          .key != null and
          .key != ""
        )
      )
    else .
    end
  )
;

# Converts decimal string to number.
def to_int:
  if . == null then . else .|tonumber end
;

# Converts "1" / "0" to boolean.
def to_bool:
  if . == null then . else
    if . == "1" then true else false end
  end
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

# Replace config in input.
. + {
  "coinbase": (if .coinbase == null then "0x0000000000000000000000000000000000000000" else .coinbase end)|to_hex,
  "difficulty": (if .difficulty == null then "0x00" else .difficulty end)|to_hex|infix_zeros_to_length(2;8),
  "extraData": (if .extraData == null then "0x00" else .extraData end)|to_hex,
  "gasLimit": (if .gasLimit == null then "0x00" else .gasLimit end)|to_hex|infix_zeros_to_length(2;8),
  "mixHash": (if .mixHash == null then "0x00" else .mixHash end)|to_hex|infix_zeros_to_length(2;66),
  "nonce": (if .nonce == null then "0x00" else .nonce end)|to_hex,
  "parentHash": (if .parentHash == null then "0x0000000000000000000000000000000000000000000000000000000000000000" else .parentHash end)|to_hex,
  "timestamp": (if .timestamp == null then "0x00" else .timestamp end)|to_hex,
  "config": {
    "ethash": (if env.HIVE_CLIQUE_PERIOD then null else {} end),
    "clique": (if env.HIVE_CLIQUE_PERIOD == null then null else {
      "period": env.HIVE_CLIQUE_PERIOD|to_int,
    } end),
    "chainId": (if env.HIVE_CHAIN_ID == null then 1 else env.HIVE_CHAIN_ID|to_int end),
    "homesteadBlock": env.HIVE_FORK_HOMESTEAD|to_int,
    "daoForkBlock": env.HIVE_FORK_DAO_BLOCK|to_int,
    "daoForkSupport": env.HIVE_FORK_DAO_VOTE|to_bool,
    "eip150Block": env.HIVE_FORK_TANGERINE|to_int,
    "eip150Hash": env.HIVE_FORK_TANGERINE_HASH,
    "eip155Block": env.HIVE_FORK_SPURIOUS|to_int,
    "eip158Block": env.HIVE_FORK_SPURIOUS|to_int,
    "byzantiumBlock": env.HIVE_FORK_BYZANTIUM|to_int,
    "constantinopleBlock": env.HIVE_FORK_CONSTANTINOPLE|to_int,
    "petersburgBlock": env.HIVE_FORK_PETERSBURG|to_int,
    "istanbulBlock": env.HIVE_FORK_ISTANBUL|to_int,
    "muirGlacierBlock": env.HIVE_FORK_MUIR_GLACIER|to_int,
    "berlinBlock": env.HIVE_FORK_BERLIN|to_int,
    "londonBlock": env.HIVE_FORK_LONDON|to_int,
    "terminalTotalDifficulty": env.HIVE_TERMINAL_TOTAL_DIFFICULTY|to_int,
  }|remove_empty
}
