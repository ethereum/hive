# Removes all empty keys and values in input.
def remove_empty:
  . | walk(
    if type == "object" then
      with_entries(
        select(
          .value != null and
          .value != "" and
          .value != [] and
          .value != {} and
          .key != null and
          .key != "" and
          .key != [] and
          .key != {}
        )
      )
    else .
    end
  );

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

{
  "version": "1",
  "accounts": (.alloc|with_entries(.key|="0x"+.)),
  "genesis": {
    "author": .coinbase,
    "difficulty": .difficulty|to_hex,
    "extraData": .extraData,
    "gasLimit": .gasLimit|to_hex,
    "nonce": .nonce|infix_zeros_to_length(2;18),
    "timestamp": .timestamp|to_hex,
  },
  "params": {
    "miningMethod": (if env.HIVE_SKIP_POW != null then "NoProof" else "ethash" end),
    "chainId": (if env.HIVE_CHAIN_ID != null then env.HIVE_CHAIN_ID|to_hex else "0x1" end),
    "frontierForkBlock": "0x0",
    "homesteadForkBlock": env.HIVE_FORK_HOMESTEAD|to_hex,
    "DAOForkBlock": env.HIVE_FORK_DAO_BLOCK|to_hex,
    "EIP150ForkBlock": env.HIVE_FORK_TANGERINE|to_hex,
    "EIP158ForkBlock": env.HIVE_FORK_SPURIOUS|to_hex,
    "byzantiumForkBlock": env.HIVE_FORK_BYZANTIUM|to_hex,
    "constantinopleForkBlock": env.HIVE_FORK_CONSTANTINOPLE|to_hex,
    "petersburgForkBlock": env.HIVE_FORK_PETERSBURG|to_hex,
    "istanbulForkBlock": env.HIVE_FORK_ISTANBUL|to_hex,
    "muirglacierForkBlock": env.HIVE_FORK_MUIR_GLACIER|to_hex,
  },
}|remove_empty
