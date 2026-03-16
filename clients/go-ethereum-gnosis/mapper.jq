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

# Rename uncleHash to sha3Uncles if it exists
. | if has("uncleHash") then
  . + {"sha3Uncles": .uncleHash} | del(.uncleHash)
else
  .
end |
# Replace config in input.
. + {
  "config": {
    "chainId": (if env.HIVE_CHAIN_ID then env.HIVE_CHAIN_ID|to_int else 100 end),
    "consensus": "aura",
    "homesteadBlock": env.HIVE_FORK_HOMESTEAD|to_int,
    "eip150Block": env.HIVE_FORK_TANGERINE|to_int,
    "eip155Block": env.HIVE_FORK_SPURIOUS|to_int,
    "byzantiumBlock": env.HIVE_FORK_BYZANTIUM|to_int,
    "constantinopleBlock": env.HIVE_FORK_CONSTANTINOPLE|to_int,
    "petersburgBlock": env.HIVE_FORK_PETERSBURG|to_int,
    "istanbulBlock": env.HIVE_FORK_ISTANBUL|to_int,
    "berlinBlock": env.HIVE_FORK_BERLIN|to_int,
    "londonBlock": env.HIVE_FORK_LONDON|to_int,
    "burntContract": {
      "0": "0x1559000000000000000000000000000000000000"
    },
    "terminalTotalDifficulty": 0,
    "terminalTotalDifficultyPassed": true,
    "shanghaiTime": env.HIVE_SHANGHAI_TIMESTAMP|to_int,
    "cancunTime": env.HIVE_CANCUN_TIMESTAMP|to_int,
    "pragueTime": env.HIVE_PRAGUE_TIMESTAMP|to_int,
    "osakaTime": env.HIVE_OSAKA_TIMESTAMP|to_int,
    "amsterdamTime": env.HIVE_AMSTERDAM_TIMESTAMP|to_int,
    "blobSchedule": {
      "cancun": {
        "target": (if env.HIVE_CANCUN_BLOB_TARGET then env.HIVE_CANCUN_BLOB_TARGET|to_int else 1 end),
        "max": (if env.HIVE_CANCUN_BLOB_MAX then env.HIVE_CANCUN_BLOB_MAX|to_int else 2 end),
        "baseFeeUpdateFraction": (if env.HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION|to_int else 1112826 end)
      },
      "prague": {
        "target": (if env.HIVE_PRAGUE_BLOB_TARGET then env.HIVE_PRAGUE_BLOB_TARGET|to_int else 1 end),
        "max": (if env.HIVE_PRAGUE_BLOB_MAX then env.HIVE_PRAGUE_BLOB_MAX|to_int else 2 end),
        "baseFeeUpdateFraction": (if env.HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION|to_int else 1112826 end)
      },
      "osaka": {
        "target": (if env.HIVE_OSAKA_BLOB_TARGET then env.HIVE_OSAKA_BLOB_TARGET|to_int else 1 end),
        "max": (if env.HIVE_OSAKA_BLOB_MAX then env.HIVE_OSAKA_BLOB_MAX|to_int else 2 end),
        "baseFeeUpdateFraction": (if env.HIVE_OSAKA_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_OSAKA_BLOB_BASE_FEE_UPDATE_FRACTION|to_int else 1112826 end)
      }
    },
    "depositContractAddress": "0xbabe2bed00000000000000000000000000000003",
    "minBlobGasPrice": 1000000000,
    "maxBlobGasPerBlock": 262144,
    "targetBlobGasPerBlock": 131072,
    "blobGasPriceUpdateFraction": 1112826,
    "aura": {
      "stepDuration": 5,
      "blockReward": 0,
      "maximumUncleCountTransition": 0,
      "maximumUncleCount": 0,
      "validators": {
        "multi": {
          "0": {
            "list": [
              "0x14747a698Ec1227e6753026C08B29b4d5D3bC484"
            ]
          }
        }
      },
      "blockRewardContractAddress": "0x2000000000000000000000000000000000000001",
      "blockRewardContractTransition": 0,
      "blockRewardContractTransitions": {
        "9186425": "0x481c034c6d9441db23ea48de68bcae812c5d39ba"
      },
      "randomnessContractAddress": {
        "0": "0x3000000000000000000000000000000000000001"
      },
      "withdrawalContractAddress": "0xbabe2bed00000000000000000000000000000003",
      "twoThirdsMajorityTransition": 0,
      "posdaoTransition": 0,
      "blockGasLimitContractTransitions": {
        "0": "0x4000000000000000000000000000000000000001"
      },
      "registrar": "0x6000000000000000000000000000000000000000",
      "eip1559FeeCollectorTransition": 0,
      "eip1559FeeCollector": "0x1559000000000000000000000000000000000000"
    }
  }|remove_empty,
  "baseFeePerGas": .baseFeePerGas,
  "difficulty": "0x00",
  "gasLimit": .gasLimit,
  "alloc": (.alloc | with_entries(.key |= if startswith("0x") then . else "0x" + . end))
}
