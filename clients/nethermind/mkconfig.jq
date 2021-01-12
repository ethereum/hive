# This JQ script generates the Nethermind config file.

def keystore_config:
  if env.HIVE_CLIQUE_PRIVATEKEY == null then
    {}
  else
    { "KeyStoreConfig": { "TestNodeKey": env.HIVE_CLIQUE_PRIVATEKEY } }
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
base_config * keystore_config
