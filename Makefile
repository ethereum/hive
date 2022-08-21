hive:
	go build -o ./hive ./hive.go
.PHONY: hive

mod-tidy:
	cd simulators/optimism/l1ops && go mod tidy && cd .. && \
	cd p2p && go mod tidy && cd .. && \
	cd rpc && go mod tidy
.PHONY: mod-tidy