# Steps to try EthereumJS locally

1- Download this repository
2- Build the repository
3- Run engine simulator with merge-ethereumjs client

```shell
git clone git@github.com:cbrzn/hive.git --branch add-merge-ethereumjs
cd hive
go build .
./hive --sim engine --client merge-ethereumjs
```