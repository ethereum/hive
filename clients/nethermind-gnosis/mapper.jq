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
  "engine": {
    "authorityRound": {
      "params": {
        "stepDuration": 5,
        "blockReward": "0x0",
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
        }
      }
    }
  },
  "params": {
    # Tangerine Whistle
    "eip150Transition": env.HIVE_FORK_TANGERINE|to_hex,

    # Spurious Dragon
    "eip160Transition": env.HIVE_FORK_SPURIOUS|to_hex,
    "eip161abcTransition": env.HIVE_FORK_SPURIOUS|to_hex,
    "eip161dTransition": env.HIVE_FORK_SPURIOUS|to_hex,
    "eip155Transition": env.HIVE_FORK_SPURIOUS|to_hex,
    "maxCodeSizeTransition": env.HIVE_FORK_SPURIOUS|to_hex,

    "gasLimitBoundDivisor": "0x400",
    "maxCodeSize": "0x6000",
    "minGasLimit": "0x1388",
    "maximumExtraDataSize": "0x20",
    "maxCodeSizeTransitionTimestamp": "0x0",
    "terminalTotalDifficulty": "0x0",
    "registrar": "0x6000000000000000000000000000000000000000",
    "transactionPermissionContract": "0x4000000000000000000000000000000000000001",
    "transactionPermissionContractTransition": "0x0",
    "feeCollector": "0x1559000000000000000000000000000000000000",
    "eip1559FeeCollectorTransition": 0,
    "eip1559BaseFeeMaxChangeDenominator": "0x8",
    "eip1559ElasticityMultiplier": "0x2",

    # Byzantium
    "eip140Transition": env.HIVE_FORK_BYZANTIUM|to_hex,
    "eip211Transition": env.HIVE_FORK_BYZANTIUM|to_hex,
    "eip214Transition": env.HIVE_FORK_BYZANTIUM|to_hex,
    "eip658Transition": env.HIVE_FORK_BYZANTIUM|to_hex,

    # Constantinople
    "eip145Transition": env.HIVE_FORK_CONSTANTINOPLE|to_hex,
    "eip1014Transition": env.HIVE_FORK_CONSTANTINOPLE|to_hex,
    "eip1052Transition": env.HIVE_FORK_CONSTANTINOPLE|to_hex,

    # Petersburg
    "eip1283Transition": (if env.HIVE_FORK_ISTANBUL|to_hex == "0x0" then "0x0" else env.HIVE_FORK_CONSTANTINOPLE|to_hex end),
    "eip1283DisableTransition": (if env.HIVE_FORK_ISTANBUL|to_hex == "0x0" then "0x0" else env.HIVE_FORK_PETERSBURG|to_hex end),

    # Istanbul
    "eip152Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip1108Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip1344Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip1884Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip2028Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip2200Transition": env.HIVE_FORK_ISTANBUL|to_hex,

    # Berlin
    "eip2565Transition": env.HIVE_FORK_BERLIN|to_hex,
    "eip2718Transition": env.HIVE_FORK_BERLIN|to_hex,
    "eip2929Transition": env.HIVE_FORK_BERLIN|to_hex,
    "eip2930Transition": env.HIVE_FORK_BERLIN|to_hex,

  
    # London
    "eip1559Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3238Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3529Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3541Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3198Transition": env.HIVE_FORK_LONDON|to_hex,
    
    # Merge
    "MergeForkIdTransition": env.HIVE_MERGE_BLOCK_ID|to_hex,

    # Shanghai
    "eip3651TransitionTimestamp": env.HIVE_SHANGHAI_TIMESTAMP|to_hex,
    "eip3855TransitionTimestamp": env.HIVE_SHANGHAI_TIMESTAMP|to_hex,
    "eip3860TransitionTimestamp": env.HIVE_SHANGHAI_TIMESTAMP|to_hex,
    "eip4895TransitionTimestamp": env.HIVE_SHANGHAI_TIMESTAMP|to_hex,

    # Cancun
    "eip4844TransitionTimestamp": env.HIVE_CANCUN_TIMESTAMP|to_hex,
    "eip4788TransitionTimestamp": env.HIVE_CANCUN_TIMESTAMP|to_hex,
    "eip1153TransitionTimestamp": env.HIVE_CANCUN_TIMESTAMP|to_hex,
    "eip5656TransitionTimestamp": env.HIVE_CANCUN_TIMESTAMP|to_hex,
    "eip6780TransitionTimestamp": env.HIVE_CANCUN_TIMESTAMP|to_hex,

    "eip4844BlobGasPriceUpdateFraction": "0x10fafa",
    "eip4844MaxBlobGasPerBlock": "0x40000",
    "eip4844MinBlobGasPrice": "0x3b9aca00",
    "eip4844TargetBlobGasPerBlock": "0x20000",
    "eip4844FeeCollectorTransitionTimestamp": "0x0",

    # Prague
    "eip2537TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip2935TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip6110TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip7002TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip7251TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip7685TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip7702TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip7623TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
    "eip7692TransitionTimestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,

    "depositContractAddress": "0xbabe2bed00000000000000000000000000000003",

    # Osaka
    "eip7594TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7823TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7825TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7883TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7918TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7951TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7939TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7934TransitionTimestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
    "eip7934MaxRlpBlockSize": "0x800000",

    # Amsterdam
    "eip7708TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
    "eip7778TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
    "eip7843TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
    "eip7928TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
    "eip7954TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
    "eip8024TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
    "eip8037TransitionTimestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,

    # Other chain parameters
    "chainID": (if env.HIVE_CHAIN_ID then env.HIVE_CHAIN_ID|to_hex else "0x64" end),

    "blobSchedule": [
      if env.HIVE_CANCUN_TIMESTAMP then {
          "timestamp": env.HIVE_CANCUN_TIMESTAMP|to_hex,
          "target": (if env.HIVE_CANCUN_BLOB_TARGET then env.HIVE_CANCUN_BLOB_TARGET|to_hex else "0x1" end),
          "max": (if env.HIVE_CANCUN_BLOB_MAX then env.HIVE_CANCUN_BLOB_MAX|to_hex else "0x2" end),
          "baseFeeUpdateFraction": (if env.HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_CANCUN_BLOB_BASE_FEE_UPDATE_FRACTION|to_hex else "0x10fafa" end)
      } else null end,
      if env.HIVE_PRAGUE_TIMESTAMP then {
          "timestamp": env.HIVE_PRAGUE_TIMESTAMP|to_hex,
          "target": (if env.HIVE_PRAGUE_BLOB_TARGET then env.HIVE_PRAGUE_BLOB_TARGET|to_hex else "0x1" end),
          "max": (if env.HIVE_PRAGUE_BLOB_MAX then env.HIVE_PRAGUE_BLOB_MAX|to_hex else "0x2" end),
          "baseFeeUpdateFraction": (if env.HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_PRAGUE_BLOB_BASE_FEE_UPDATE_FRACTION|to_hex else "0x10fafa" end)
      } else null end,
      if env.HIVE_OSAKA_TIMESTAMP then {
          "timestamp": env.HIVE_OSAKA_TIMESTAMP|to_hex,
          "target": (if env.HIVE_OSAKA_BLOB_TARGET then env.HIVE_OSAKA_BLOB_TARGET|to_hex else "0x1" end),
          "max": (if env.HIVE_OSAKA_BLOB_MAX then env.HIVE_OSAKA_BLOB_MAX|to_hex else "0x2" end),
          "baseFeeUpdateFraction": (if env.HIVE_OSAKA_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_OSAKA_BLOB_BASE_FEE_UPDATE_FRACTION|to_hex else "0x10fafa" end)
      } else null end,
      if env.HIVE_AMSTERDAM_TIMESTAMP then {
          "timestamp": env.HIVE_AMSTERDAM_TIMESTAMP|to_hex,
          "target": (if env.HIVE_AMSTERDAM_BLOB_TARGET then env.HIVE_AMSTERDAM_BLOB_TARGET|to_hex else "0x1" end),
          "max": (if env.HIVE_AMSTERDAM_BLOB_MAX then env.HIVE_AMSTERDAM_BLOB_MAX|to_hex else "0x2" end),
          "baseFeeUpdateFraction": (if env.HIVE_AMSTERDAM_BLOB_BASE_FEE_UPDATE_FRACTION then env.HIVE_AMSTERDAM_BLOB_BASE_FEE_UPDATE_FRACTION|to_hex else "0x10fafa" end)
      } else null end
    ] | map(select(. != null)) | reverse | unique_by(.timestamp)
  },
  "genesis": {
    "difficulty": .difficulty,
    "author": .coinbase,
    "timestamp": .timestamp,
    "parentHash": .parentHash,
    "extraData": .extraData,
    "gasLimit": .gasLimit,
    "baseFeePerGas": .baseFeePerGas,
    "blobGasUsed": .blobGasUsed,
    "excessBlobGas": .excessBlobGas,
    "parentBeaconBlockRoot": .parentBeaconBlockRoot,
  },
  "accounts": (.alloc | with_entries(.key |= if startswith("0x") then . else "0x" + . end))
}|remove_empty