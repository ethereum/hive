module github.com/ethereum/hive/simulators/ronin

go 1.21.3

require (
	github.com/ethereum/go-ethereum v1.13.5-0.20231031113925-bc42e88415d3
	github.com/ethereum/hive v0.0.0-00010101000000-000000000000
)

replace github.com/ethereum/hive => github.com/dnk90/hive v0.0.0-20240111083448-9ee2fc27a020

replace github.com/ethereum/go-ethereum => github.com/axieinfinity/ronin v1.10.4-0.20231115060406-4878e9bd4991

require (
	github.com/btcsuite/btcd v0.20.1-beta // indirect
	github.com/deckarep/golang-set v0.0.0-20180603214616-504e848d77ea // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	gopkg.in/inconshreveable/log15.v2 v2.0.0-20200109203555-b30bc20e4fd1 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce // indirect
)
