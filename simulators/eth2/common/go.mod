module github.com/ethereum/hive/simulators/eth2/common

go 1.18

require (
	github.com/ethereum/go-ethereum v1.10.26
	github.com/ethereum/hive v0.0.0-20221214152536-bfabd993ae7b
	github.com/golang-jwt/jwt/v4 v4.3.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/herumi/bls-eth-go-binary v1.28.1
	github.com/holiman/uint256 v1.2.1
	github.com/pkg/errors v0.9.1
	github.com/protolambda/bls12-381-util v0.0.0-20220416220906-d8552aa452c7
	github.com/protolambda/eth2api v0.0.0-20220822011642-f7735dd471e0
	github.com/protolambda/go-keystorev4 v0.0.0-20211007151826-f20444f6d564
	github.com/protolambda/zrnt v0.30.0
	github.com/protolambda/ztyp v0.2.2
	github.com/rauljordan/engine-proxy v0.0.0-20220517190449-e62b2e2f6e27
	github.com/sirupsen/logrus v1.9.0
	github.com/tyler-smith/go-bip39 v1.1.0
	github.com/wealdtech/go-eth2-util v1.8.0
	golang.org/x/exp v0.0.0-20230108222341-4b8118a2686a
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/VictoriaMetrics/fastcache v1.12.0 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.3.2 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/deckarep/golang-set/v2 v2.1.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.1.0 // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/ferranbt/fastssz v0.1.2 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/holiman/big v0.0.0-20221017200358-a027dc42d04e // indirect
	github.com/holiman/bloomfilter/v2 v2.0.3 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kilic/bls12-381 v0.1.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.2 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/rivo/uniseg v0.4.3 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/rs/cors v1.8.2 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220614013038-64ee5596c38a // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/wealdtech/go-bytesutil v1.2.0 // indirect
	github.com/wealdtech/go-eth2-types/v2 v2.8.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/crypto v0.4.0 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/rauljordan/engine-proxy => github.com/marioevz/engine-proxy v0.0.0-20220617181151-e8661eb39eea

replace github.com/ethereum/go-ethereum v1.10.26 => github.com/lightclient/go-ethereum v1.10.10-0.20230116085521-6ab6d738866f

replace github.com/protolambda/eth2api => github.com/marioevz/eth2api v0.0.0-20230214151319-641a58f39ae4
