def remove_empty:
  . | walk(
    if type == "object" then
      with_entries(
        select(
          .value != null and
          .value != "" and
          .value != [] and
          .value != {}
        )
      )
    else .
    end
  );

def to_hex:
  if . != null and startswith("0x") then . else
    if . != null then "0x"+. else . end
  end
;

def infix_zeros_to_length(s;l):
   if . != null then
     (.[0:s])+("0"*(l-(.|length)))+(.[s:l])
   else .
   end
;

.|
.alloc|=with_entries(.key|="0x"+.) |
{
  "accounts": .alloc,
  "genesis": {
    "author":.coinbase,
    "difficulty":.difficulty,
    "extraData":.extraData,
    "gasLimit":.gasLimit,
    "nonce": .nonce|infix_zeros_to_length(2;18),
    "timestamp":.timestamp,
  },
  "params": {
    "miningMethod": (if env.HIVE_SKIP_POW != null then "NoProof" else "ethash" end),
    "chainId": env.HIVE_CHAIN_ID|to_hex,
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
  "version": "1"
}|remove_empty

