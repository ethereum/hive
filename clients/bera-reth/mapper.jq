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

# Replace config in input.
. + {
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
    "arrowGlacierBlock": env.HIVE_FORK_ARROW_GLACIER|to_int,
    "grayGlacierBlock": env.HIVE_FORK_GRAY_GLACIER|to_int,
    "mergeNetsplitBlock": env.HIVE_MERGE_BLOCK_ID|to_int,
    "terminalTotalDifficulty": env.HIVE_TERMINAL_TOTAL_DIFFICULTY|to_int,
    "terminalTotalDifficultyPassed": env.HIVE_TERMINAL_TOTAL_DIFFICULTY_PASSED|to_bool,
    "shanghaiTime": env.HIVE_SHANGHAI_TIMESTAMP|to_int,
    "cancunTime": env.HIVE_CANCUN_TIMESTAMP|to_int,
    "pragueTime": env.HIVE_PRAGUE_TIMESTAMP|to_int,
    "osakaTime": env.HIVE_OSAKA_TIMESTAMP|to_int,
    "blobSchedule": {
      "cancun": {
        "target": (if env.HIVE_CANCUN_BLOB_TARGET then env.HIVE_CANCUN_BLOB_TARGET|to_int else 3 end),
        "max": (if env.HIVE_CANCUN_BLOB_MAX then env.HIVE_CANCUN_BLOB_MAX|to_int else 6 end),
        "baseFeeUpdateFraction": (if env.HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION|to_int else 3338477 end)
      },
      "prague": {
        "target": (if env.HIVE_PRAGUE_BLOB_TARGET then env.HIVE_PRAGUE_BLOB_TARGET|to_int else 6 end),
        "max": (if env.HIVE_PRAGUE_BLOB_MAX then env.HIVE_PRAGUE_BLOB_MAX|to_int else 9 end),
        "baseFeeUpdateFraction": (if env.HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION|to_int else 5007716 end)
      },
      "osaka": {
        "target": (if env.HIVE_OSAKA_BLOB_TARGET then env.HIVE_OSAKA_BLOB_TARGET|to_int else 6 end),
        "max": (if env.HIVE_OSAKA_BLOB_MAX then env.HIVE_OSAKA_BLOB_MAX|to_int else 9 end),
        "baseFeeUpdateFraction": (if env.HIVE_OSAKA_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_OSAKA_BLOB_BASE_FEE_UPDATE_FRACTION|to_int else 5007716 end)
      }
    },
    "berachain": (if env.HIVE_PRAGUE1_TIMESTAMP or env.HIVE_PRAGUE2_TIMESTAMP then {
      "prague1": (if env.HIVE_PRAGUE1_TIMESTAMP then {
        "time": env.HIVE_PRAGUE1_TIMESTAMP|to_int,
        "baseFeeChangeDenominator": (if env.HIVE_PRAGUE1_BASE_FEE_CHANGE_DENOMINATOR then env.HIVE_PRAGUE1_BASE_FEE_CHANGE_DENOMINATOR|to_int else 48 end),
        "minimumBaseFeeWei": (if env.HIVE_PRAGUE1_MINIMUM_BASE_FEE_WEI then env.HIVE_PRAGUE1_MINIMUM_BASE_FEE_WEI|to_int else 1000000000 end),
        "polDistributorAddress": (env.HIVE_PRAGUE1_POL_DISTRIBUTOR_ADDRESS // "0x4200000000000000000000000000000000000042")
      } else null end),
      "prague2": (if env.HIVE_PRAGUE2_TIMESTAMP then {
        "time": env.HIVE_PRAGUE2_TIMESTAMP|to_int,
        "minimumBaseFeeWei": (if env.HIVE_PRAGUE2_MINIMUM_BASE_FEE_WEI then env.HIVE_PRAGUE2_MINIMUM_BASE_FEE_WEI|to_int else 0 end)
      } else null end)
    } else null end),
    "depositContractAddress": "0x0000000000000000000000000000000000000000"
  }|remove_empty
}