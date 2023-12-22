package taiko

// Taiko environment variables
//  - HIVE_TAIKO_BOOTNODE                             enode URL of the remote bootstrap node for l2 node
//  - HIVE_TAIKO_L1_RPC_ENDPOINT                      rpc endpoint of the L1 node
//  - HIVE_TAIKO_L2_RPC_ENDPOINT                      rpc endpoint of the L2 node
//  - HIVE_TAIKO_L2_ENGINE_ENDPOINT                   engine endpoint of the l2 node
//  - HIVE_TAIKO_L1_ROLLUP_ADDRESS                    rollup address of the L1 node
//  - HIVE_TAIKO_L2_ROLLUP_ADDRESS                    rollup address of the L2 node
//  - HIVE_TAIKO_PROPOSER_PRIVATE_KEY                 private key of the proposer
//  - HIVE_TAIKO_SUGGESTED_FEE_RECIPIENT              suggested fee recipient
//  - HIVE_TAIKO_PROPOSE_INTERVAL                     propose interval
//  - HIVE_TAIKO_THROWAWAY_BLOCK_BUILDER_PRIVATE_KEY  private key of the throwaway block builder
//  - HIVE_TAIKO_L1_CHAIN_ID                          L1 chain id
//  - HIVE_TAIKO_PROVER_PRIVATE_KEY                   private key of the prover

// taiko environment variables constants
const (
	// hive common
	envNetworkID    = "HIVE_NETWORK_ID"
	envBootNode     = "HIVE_BOOTNODE"
	envCliquePeriod = "HIVE_CLIQUE_PERIOD"
	envNodeType     = "HIVE_NODETYPE"
	envLogLevel     = "HIVE_LOGLEVEL"

	// taiko common
	envTaikoRole            = "HIVE_TAIKO_ROLE"
	envTaikoL1ChainID       = "HIVE_TAIKO_L1_CHAIN_ID"
	envTaikoJWTSecret       = "HIVE_TAIKO_JWT_SECRET"
	envTaikoL1HTTPEndpoint  = "HIVE_TAIKO_L1_HTTP_ENDPOINT"
	envTaikoL2HTTPEndpoint  = "HIVE_TAIKO_L2_HTTP_ENDPOINT"
	envTaikoL1WSEndpoint    = "HIVE_TAIKO_L1_WS_ENDPOINT"
	envTaikoL2WSEndpoint    = "HIVE_TAIKO_L2_WS_ENDPOINT"
	envTaikoL1RollupAddress = "HIVE_TAIKO_L1_ROLLUP_ADDRESS"
	envTaikoL2RollupAddress = "HIVE_TAIKO_L2_ROLLUP_ADDRESS"

	// driver
	envTaikoL2EngineEndpoint                = "HIVE_TAIKO_L2_ENGINE_ENDPOINT"
	envTaikoThrowawayBlockBuilderPrivateKey = "HIVE_TAIKO_THROWAWAY_BLOCK_BUILDER_PRIVATE_KEY"
	evnTaikoEnableL2P2P                     = "HIVE_TAIKO_ENABLE_L2_P2P"

	// proposer
	envTaikoProposeInterval              = "HIVE_TAIKO_PROPOSE_INTERVAL"
	envTaikoProposerPrivateKey           = "HIVE_TAIKO_PROPOSER_PRIVATE_KEY"
	envTaikoSuggestedFeeRecipient        = "HIVE_TAIKO_SUGGESTED_FEE_RECIPIENT"
	envTaikoProduceInvalidBlocksInterval = "HIVE_TAIKO_PRODUCE_INVALID_BLOCKS_INTERVAL"

	// prover
	envTaikoProverPrivateKey = "HIVE_TAIKO_PROVER_PRIVATE_KEY"

	// deployer
	envTaikoL1DeployerAddress  = "HIVE_TAIKO_L1_DEPLOYER_ADDRESS"
	envTaikoL2GenesisBlockHash = "HIVE_TAIKO_L2_GENESIS_BLOCK_HASH"
	envTaikoMainnetUrl         = "HIVE_TAIKO_MAINNET_URL"
	envTaikoPrivateKey         = "HIVE_TAIKO_PRIVATE_KEY"
	envTaikoL2ChainID          = "HIVE_TAIKO_L2_CHAIN_ID"
)
