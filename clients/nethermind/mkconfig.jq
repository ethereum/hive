# This JQ script generates the Nethermind config file.

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

def keystore_config:
  if env.HIVE_CLIQUE_PRIVATEKEY == null then
    {}
  else
    { "KeyStoreConfig": { "TestNodeKey": env.HIVE_CLIQUE_PRIVATEKEY } }
  end
;

def merge_config:
  if env.HIVE_TERMINAL_TOTAL_DIFFICULTY != null then
    {
      "Merge": {
        "Enabled": true,
        "TerminalTotalDifficulty": env.HIVE_TERMINAL_TOTAL_DIFFICULTY,
        "TerminalBlockHash": env.HIVE_TERMINAL_BLOCK_HASH,
        "TerminalBlockNumber": env.HIVE_TERMINAL_BLOCK_NUMBER,
      } | remove_empty
    }
  else
    {}
  end
;

def json_rpc_config:
  if env.HIVE_TERMINAL_TOTAL_DIFFICULTY != null then
    {
      "JsonRpc": {
        "JwtSecretFile": "/jwt.secret",
        "EnabledModules": ["Debug", "Eth", "Subscribe", "Trace", "TxPool", "Web3", "Personal", "Proof", "Net", "Parity", "Health", "Admin"],
        "AdditionalRpcUrls": ["http://0.0.0.0:8550|http;ws|debug;net;eth;subscribe;engine;web3;client;admin|no-auth", "http://0.0.0.0:8551|http;ws|debug;net;eth;subscribe;engine;web3;client;admin"]
      }
    }
  else
    {
      "JsonRpc": {
        "EnabledModules": ["Debug", "Eth", "Subscribe", "Trace", "TxPool", "Web3", "Personal", "Proof", "Net", "Parity", "Health", "Admin"]
      }
    }
  end
;

def sync_config:
  {
    "Sync": {
      "SnapSync": (env.HIVE_NODETYPE == "snap"),
    },
  }
;

def txpool_config:
  if env.HIVE_CANCUN_TIMESTAMP != null then
    {
      "TxPool": {
        "BlobsSupport": "StorageWithReorgs"
      }
    }
  else
    {}
  end
;

def base_config:
  {
    "Init": {
      "WebSocketsEnabled": true,
      "IsMining": (env.HIVE_MINER != null),
      "UseMemDb": true,
      "ChainSpecPath": "/chainspec/test.json",
      "BaseDbPath": "nethermind_db/hive",
      "LogFileName": "/hive.logs.txt"
    },
    "JsonRpc": {
      "Enabled": true,
      "Host": "0.0.0.0",
      "Port": 8545,
      "GasCap": 50000000,
      "WebSocketsPort": 8546,
    },
    "Network": {
      "DiscoveryPort": 30303,
      "P2PPort": 30303,
      "ExternalIp": "127.0.0.1",
    },
    "Hive": {
      "ChainFile": "/chain.rlp",
      "GenesisFilePath": "/genesis.json",
      "BlocksDir": "/blocks",
      "KeysDir": "/keys"
    },
    "Sync": {
      "SnapServingEnabled": true,
    },
    "Discovery": {
      "DiscoveryVersion": "All"
    },
  }
;

# This is the main expression that outputs the config.
base_config * keystore_config * merge_config * json_rpc_config * sync_config * txpool_config
