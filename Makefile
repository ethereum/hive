hive:
	go build -o ./hive ./hive.go
.PHONY: hive

mod-tidy:
	cd simulators/optimism/l1ops && go mod tidy && cd .. && \
	cd p2p && go mod tidy && cd .. && \
	cd rpc && go mod tidy && \
	cd ../../../ && go mod tidy
.PHONY: mod-tidy

contracts:
	cd contracts && \
		solc --bin --abi --overwrite -o . ./SimpleERC20.sol && \
		solc --bin --abi --overwrite -o . ./Failure.sol
.PHONY: contracts

bindings: contracts
	abigen \
		--abi ./contracts/SimpleERC20.abi \
		--bin ./contracts/SimpleERC20.bin \
		--pkg bindings \
		--type "SimpleERC20" \
		--out ./optimism/bindings/simple_erc20.go
	abigen \
		--abi ./contracts/Failure.abi \
		--bin ./contracts/Failure.bin \
		--pkg bindings \
		--type "Failure" \
		--out ./optimism/bindings/failure.go
.PHONY: bindings