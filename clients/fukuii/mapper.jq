# mapper.jq — Convert geth-format genesis.json to fukuii-compatible genesis.json
# Pass through all header fields that affect the genesis block hash
{
  "difficulty": .difficulty,
  "gasLimit": .gasLimit,
  "nonce": (.nonce // "0x0000000000000000"),
  "timestamp": (.timestamp // "0x0"),
  "coinbase": (.coinbase // "0x0000000000000000000000000000000000000000"),
  "mixHash": (.mixHash // .mixhash // "0x0000000000000000000000000000000000000000000000000000000000000000"),
  "extraData": (.extraData // "0x"),
  "gasUsed": (.gasUsed // "0x0"),
  "parentHash": (.parentHash // "0x0000000000000000000000000000000000000000000000000000000000000000"),
  "baseFeePerGas": .baseFeePerGas,
  "excessBlobGas": .excessBlobGas,
  "blobGasUsed": .blobGasUsed,
  "slotNumber": .slotNumber,
  "alloc": ((.alloc // {}) | to_entries | map({
    key: .key,
    value: {
      balance: (.value.balance // "0"),
      code: .value.code,
      nonce: .value.nonce,
      storage: .value.storage
    } | with_entries(select(.value != null))
  }) | from_entries)
} | with_entries(select(.value != null))
