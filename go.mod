module github.com/ethereum/hive

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/ethereum/go-ethereum v1.10.20
	github.com/ethereum/hive/hiveproxy v0.0.0-20220708193637-ec524d7345a1
	github.com/fsouza/go-dockerclient v1.8.1
	github.com/gorilla/mux v1.8.0
	gopkg.in/inconshreveable/log15.v2 v2.0.0-20200109203555-b30bc20e4fd1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	golang.org/x/crypto v0.0.0-20211202192323-5770296d904e // indirect
)

replace github.com/ethereum/hive/hiveproxy => ./hiveproxy
