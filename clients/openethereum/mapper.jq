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

# This gives the consensus engine definition for the ethash engine.
def ethash_engine:
  {
    "Ethash": {
      "params": {
        "minimumDifficulty": "0x20000",
        "difficultyBoundDivisor": "0x800",
        "durationLimit": "0x0d",
        "homesteadTransition": env.HIVE_FORK_HOMESTEAD|to_hex,
        "eip100bTransition": env.HIVE_FORK_BYZANTIUM|to_hex,
        "daoHardforkTransition": env.HIVE_FORK_DAO_BLOCK|to_hex,
        "daoHardforkBeneficiary": "0xbf4ed7b27f1d666546e30d74d50d173d20bca754",
        "blockReward": {
          "0x0": "0x4563918244F40000",
          (env.HIVE_FORK_BYZANTIUM|to_hex//""): "0x29A2241AF62C0000",
          (env.HIVE_FORK_CONSTANTINOPLE|to_hex//""): "0x1BC16D674EC80000",
        },
        "difficultyBombDelays": {
          (env.HIVE_FORK_BYZANTIUM|to_hex//""): 3000000,
          (env.HIVE_FORK_CONSTANTINOPLE|to_hex//""): 2000000,
          (env.HIVE_FORK_MUIR_GLACIER|to_hex//""): 4000000,
          (env.HIVE_FORK_LONDON|to_hex//""): 700000,
        }
      }
    }
  }
;

# This gives the consensus engine definition for the clique PoA engine.
def clique_engine:
  {
    "clique": {
       "params": {
         "period": env.HIVE_CLIQUE_PERIOD|tonumber,
         "epoch": 30000,
         "blockReward": "0x0"
       }
    }
  }
;

{
  "name": "Hive",
  "engine": (if env.HIVE_CLIQUE_PERIOD then clique_engine else ethash_engine end),
  "genesis": {
    "seal": {
      "ethereum": {
         "nonce": (.nonce|infix_zeros_to_length(2;18) // "0x0000000000000000"),
         "mixHash": (.mixHash // "0x0000000000000000000000000000000000000000000000000000000000000000"),
      },
    },
    "difficulty": .difficulty,
    "author": .coinbase,
    "timestamp": .timestamp,
    "parentHash": .parentHash,
    "extraData": .extraData,
    "gasLimit": .gasLimit,
    "baseFeePerGas": .baseFeePerGas,
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
    "maxCodeSize": "0x6000",

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
    "eip1283ReenableTransition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip1344Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip1884Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip2028Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    "eip1706Transition": env.HIVE_FORK_ISTANBUL|to_hex,
    # "eip2200AdvanceTransition": env.HIVE_FORK_ISTANBUL|to_hex,

    # Berlin
    "eip2929Transition": env.HIVE_FORK_BERLIN|to_hex,
    "eip2930Transition": env.HIVE_FORK_BERLIN|to_hex,

    # London
    "eip1559Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3198Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3541Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip3529Transition": env.HIVE_FORK_LONDON|to_hex,
    "eip1559BaseFeeMaxChangeDenominator": "0x8",
    "eip1559ElasticityMultiplier": "0x2",
    "eip1559BaseFeeInitialValue": "0x3B9ACA00",

    # Other chain parameters.
    "networkID": (env.HIVE_NETWORK_ID|to_hex // "0x539"),
    "chainID": (env.HIVE_CHAIN_ID|to_hex // "0x539"),
    "accountStartNonce": "0x0",
    "maximumExtraDataSize": "0x20",
    "minGasLimit": "0x0",
    "gasLimitBoundDivisor": "0x400",
    "registrar": "0x0000000000000000000000000000000000000000",
  },
  "accounts": ((.alloc|with_entries(.key|="0x"+.)) * {
    "0x0000000000000000000000000000000000000001": {
      "builtin": {
        "name": "ecrecover",
        "pricing": {"linear": {"base": 3000, "word": 0}}
      }
    },
    "0x0000000000000000000000000000000000000002": {
      "builtin": {
        "name": "sha256",
        "pricing": {"linear": {"base": 60, "word": 12}}
      }
    },
    "0x0000000000000000000000000000000000000003": {
      "builtin": {
        "name": "ripemd160",
        "pricing": {"linear": {"base": 600, "word": 120}}
      }
    },
    "0x0000000000000000000000000000000000000004": {
      "builtin": {
        "name": "identity",
        "pricing": {"linear": {"base": 15, "word": 3}}
      }
    },
    "0x0000000000000000000000000000000000000005": {
      "builtin": {
        "name": "modexp",
        "pricing":{
         (env.HIVE_FORK_BYZANTIUM|to_hex // ""): {
            "info": "EIP-198: Big integer modular exponentiation. Byzantium hardfork",
            "price": {"modexp": {"divisor": 20}}
         },
         (env.HIVE_FORK_BERLIN|to_hex // ""): {
            "info": "EIP-2565: Big integer modular exponentiation. Berlin hardfork",
            "price": {"modexp2565": {}}
         }
        }
      }
    },
    "0x0000000000000000000000000000000000000006": {
      "builtin": {
        "name": "alt_bn128_add",
        "activate_at": (env.HIVE_FORK_BYZANTIUM|to_hex // "0x7fffffffffffff"),
        "pricing": {
          (env.HIVE_FORK_BYZANTIUM|to_hex // ""): {
            "price": {"alt_bn128_const_operations": {"price": 500}}
          },
          (env.HIVE_FORK_ISTANBUL|to_hex // ""): {
            "price": {"alt_bn128_const_operations": {"price": 150}}
          }
        }
      }
    },
    "0x0000000000000000000000000000000000000007": {
      "builtin": {
        "name": "alt_bn128_mul",
        "activate_at": (env.HIVE_FORK_BYZANTIUM|to_hex // "0x7fffffffffffff"),
        "pricing": {
          (env.HIVE_FORK_BYZANTIUM|to_hex // ""): {
            "price": {"alt_bn128_const_operations": {"price": 40000}}
          },
          (env.HIVE_FORK_ISTANBUL|to_hex // ""): {
            "price": {"alt_bn128_const_operations": {"price": 6000}}
          }
        }
      }
    },
    "0x0000000000000000000000000000000000000008": {
      "builtin": {
        "name": "alt_bn128_pairing",
        "activate_at": (env.HIVE_FORK_BYZANTIUM|to_hex // "0x7fffffffffffff"),
        "pricing": {
          (env.HIVE_FORK_BYZANTIUM|to_hex // ""): {
            "price": {"alt_bn128_pairing": {"base": 100000, "pair": 80000}}
          },
          (env.HIVE_FORK_ISTANBUL|to_hex // ""): {
            "price": {"alt_bn128_pairing": {"base": 45000, "pair": 34000}}
          }
        }
      }
    },
    "0x0000000000000000000000000000000000000009": {
      "builtin": {
        "name": "blake2_f",
        "activate_at": (env.HIVE_FORK_ISTANBUL|to_hex // "0x7fffffffffffff"),
        "pricing": {"blake2_f": {"gas_per_round": 1}}
      }
    },
  }),
}|remove_empty
