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

# Replace 'config' section in input JSON.
. + {
  "config": {
    "ethash": (if env.HIVE_CLIQUE_PERIOD then null else {} end),
    "clique": (if env.HIVE_CLIQUE_PERIOD == null then null else {
      "blockperiodseconds": env.HIVE_CLIQUE_PERIOD|to_int,
      "epochlength": 30000,
    } end),
    "chainID": env.HIVE_CHAIN_ID|to_int,
    "homesteadBlock": env.HIVE_FORK_HOMESTEAD|to_int,
    "daoForkBlock": env.HIVE_FORK_DAO_BLOCK|to_int,
    "eip150Block": env.HIVE_FORK_TANGERINE|to_int,
    "eip150Hash": env.HIVE_FORK_TANGERINE_HASH,
    "eip155Block": env.HIVE_FORK_SPURIOUS|to_int,
    "eip158Block": env.HIVE_FORK_SPURIOUS|to_int,
    "byzantiumBlock": env.HIVE_FORK_BYZANTIUM|to_int,
    "constantinopleBlock": env.HIVE_FORK_CONSTANTINOPLE|to_int,
    "constantinopleFixBlock": env.HIVE_FORK_PETERSBURG|to_int,
    "istanbulBlock": env.HIVE_FORK_ISTANBUL|to_int,
    "muirGlacierBlock": env.HIVE_FORK_MUIR_GLACIER|to_int,
    "berlinBlock": env.HIVE_FORK_BERLIN|to_int,
    "londonBlock": env.HIVE_FORK_LONDON|to_int,
    "arrowGlacierBlock": env.HIVE_FORK_ARROW_GLACIER|to_int,
    "grayGlacierBlock": env.HIVE_FORK_GRAY_GLACIER|to_int,
    "mergeNetsplitBlock": env.HIVE_MERGE_BLOCK_ID|to_int,
    "terminalTotalDifficulty": env.HIVE_TERMINAL_TOTAL_DIFFICULTY|to_int,
    "shanghaiTime": env.HIVE_SHANGHAI_TIMESTAMP|to_int,
    "cancunTime": env.HIVE_CANCUN_TIMESTAMP|to_int,
  }|remove_empty
}
