# This JQ script generates the Nethermind config file.

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
      }
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
        "EnabledModules": ["Eth", "Subscribe", "Trace", "TxPool", "Web3", "Personal", "Proof", "Net", "Parity", "Health"],
        "AdditionalRpcUrls": ["http://0.0.0.0:8550|http;ws|net;eth;subscribe;engine;web3;client|no-auth", "http://0.0.0.0:8551|http;ws|net;eth;subscribe;engine;web3;client"]
      }
    }
  else
    {
      "JsonRpc": {
        "EnabledModules": ["Eth", "Subscribe", "Trace", "TxPool", "Web3", "Personal", "Proof", "Net", "Parity", "Health"]
      }
    }
  end
;

def base_config:
  {
    "Init": {
      "PubSubEnabled": true,
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
  }
;

# This is the main expression that outputs the config.
base_config * keystore_config * merge_config * json_rpc_config
