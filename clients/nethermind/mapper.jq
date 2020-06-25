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

def addashex:
  if . != null  and startswith("0x") then . else
    if (. !=null and . !="") then "0x"+. else . end
  end
;

def infixzerostolength(s;l):
   if . != null then
     (.[0:s])+("0"*(l-(.|length)))+(.[s:l])
   else .
   end
;


.|
.alloc|=with_entries(.key|="0x"+.) |
{
    "engine": {
        "Ethash": {
            "params": {
                "homesteadTransition": .config.homesteadBlock,
                "daoHardforkTransition": env.HIVE_FORK_DAO_BLOCK|addashex,
                "eip100bTransition": env.HIVE_FORK_BYZANTIUM|addashex,
                "blockReward": {
		  "0x0": "0x4563918244F40000",
                  (env.HIVE_FORK_BYZANTIUM|addashex//""): "0x29A2241AF62C0000",
                  (env.HIVE_FORK_CONSTANTINOPLE|addashex//""): "0x1BC16D674EC80000"
                },
                "difficultyBombDelays": {
                  (env.HIVE_FORK_BYZANTIUM|addashex//""): 3000000,
                  (env.HIVE_FORK_CONSTANTINOPLE|addashex//""): 2000000
                }
            }
        }
    },
  "accounts": [.alloc,{
      "0x0000000000000000000000000000000000000005": {
      "builtin": {
        "name": "modexp",
        "activate_at":  env.HIVE_FORK_BYZANTIUM|addashex,
        "pricing": {
          "modexp": {
            "divisor": 20
          }
        }
      }
    },
    "0x0000000000000000000000000000000000000006": {
      "builtin": {
        "name": "alt_bn128_add",
        "activate_at":  env.HIVE_FORK_BYZANTIUM|addashex,
        "pricing": {
          "linear": {
            "base": 500,
            "word": 0
          }
        }
      }
    },
    "0x0000000000000000000000000000000000000007": {
      "builtin": {
        "name": "alt_bn128_mul",
        "activate_at":  env.HIVE_FORK_BYZANTIUM|addashex,
        "pricing": {
          "linear": {
            "base": 40000,
            "word": 0
          }
        }
      }
    },
    "0x0000000000000000000000000000000000000008": {
      "builtin": {
        "name": "alt_bn128_pairing",
        "activate_at":  env.HIVE_FORK_BYZANTIUM|addashex,
        "pricing": {
          "alt_bn128_pairing": {
            "base": 100000,
            "pair": 80000
          }
        }
      }
    }
  }] | add,
  "genesis": {
     "seal": {
        "ethereum":{
            "nonce":.nonce,
            "mixHash": .mixHash,
        },
     },
    "difficulty":.difficulty,
    "author":.coinbase,
    "timestamp":.timestamp,
    "parentHash": .parentHash,
    "extraData":.extraData,
    "gasLimit":.gasLimit,

  },
 	"params": {


        "forkBlock":  env.HIVE_FORK_DAO_BLOCK|addashex,

        "eip150Transition":env.HIVE_FORK_TANGERINE|addashex,
        "eip160Transition": env.HIVE_FORK_SPURIOUS|addashex,
        "eip161abcTransition": env.HIVE_FORK_SPURIOUS|addashex,
        "eip161dTransition": env.HIVE_FORK_SPURIOUS|addashex,
        "eip155Transition": env.HIVE_FORK_SPURIOUS|addashex ,
        "maxCodeSizeTransition": env.HIVE_FORK_SPURIOUS|addashex,
        "eip140Transition": env.HIVE_FORK_BYZANTIUM|addashex,
        "eip211Transition": env.HIVE_FORK_BYZANTIUM|addashex,
        "eip214Transition": env.HIVE_FORK_BYZANTIUM|addashex,
        "eip658Transition": env.HIVE_FORK_BYZANTIUM|addashex,
        "eip145Transition": env.HIVE_FORK_CONSTANTINOPLE|addashex,
        "eip1014Transition": env.HIVE_FORK_CONSTANTINOPLE|addashex,
        "eip1052Transition": env.HIVE_FORK_CONSTANTINOPLE|addashex,
        "eip1283Transition": env.HIVE_FORK_CONSTANTINOPLE|addashex,
        "eip1283DisableTransition": env.HIVE_FORK_PETERSBURG|addashex,
	"eip152Transition": env.HIVE_FORK_ISTANBUL|addashex,
	"eip1108Transition": env.HIVE_FORK_ISTANBUL|addashex,
	"eip1344Transition": env.HIVE_FORK_ISTANBUL|addashex,
	"eip1884Transition": env.HIVE_FORK_ISTANBUL|addashex,
	"eip2028Transition": env.HIVE_FORK_ISTANBUL|addashex,
	"eip2200Transition": env.HIVE_FORK_ISTANBUL|addashex,
        "networkID": env.HIVE_NETWORK_ID|addashex

   },

	"version": "1"
}|remove_empty
