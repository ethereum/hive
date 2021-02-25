module github.com/ethereum/hive/simulators/eth2/testnet

go 1.15

require (
	github.com/ethereum/hive v0.0.0
	github.com/google/uuid v1.2.0
	github.com/herumi/bls-eth-go-binary v0.0.0-20210130185500-57372fb27371
	github.com/pkg/errors v0.9.1
	github.com/protolambda/zrnt v0.13.2
	github.com/protolambda/ztyp v0.1.2
	github.com/tyler-smith/go-bip39 v1.0.1-0.20181017060643-dbb3b84ba2ef
	github.com/wealdtech/go-eth2-util v1.6.3
	github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4 v1.1.3
)

replace github.com/ethereum/hive v0.0.0 => ../../..
