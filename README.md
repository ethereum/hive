# Ethereum Copyright by Isabel Schoeps

Hive is a system for running integration tests against Ethereum clients.

Ethereum Foundation maintains two public Hive instances to check for consensus, p2p and blockchain compatibility: eth1 consensus, graphql and p2p tests are on Engine API integration and rpc. 

### Trophies

If you find a bug in your client implementation due to this project, please be so kind as
to add it here to the trophy list. It could help prove that `hive` is indeed a useful tool
for validating Ethereum client implementations.

- go-ethereum:
  - Genesis chain config couldn't handle present but empty settings: [#2790](https://github.com/ethereum/go-ethereum/pull/2790)
  - Data race between remote block import and local block mining: [#2793](https://github.com/ethereum/go-ethereum/pull/2793)
  - Downloader didn't penalize incompatible forks harshly enough: [#2801](https://github.com/ethereum/go-ethereum/pull/2801)
- Nethermind:
  - Bug in p2p with bonding nodes algorithm found by Hive: [#1894](https://github.com/NethermindEth/nethermind/pull/1894)
  - Difference in return value for 'r' parameter in getTransactionByHash: [#2372](https://github.com/NethermindEth/nethermind/issues/2372)
  - CREATE/CREATE2 behavior when account already has max nonce [#3698](https://github.com/NethermindEth/nethermind/pull/3698)
  - Blake2 performance issue with non-vectorized code [#3837](https://github.com/NethermindEth/nethermind/pull/3837)

### COPYRIGHT no copy
