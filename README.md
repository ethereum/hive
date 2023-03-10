# hive - Ethereum end-to-end test harness

Hive is a system for running integration tests against Ethereum clients.

Ethereum Foundation maintains two public Hive instances to check for consensus, p2p and
blockchain compatibility:

- eth1 consensus, graphql and p2p tests are on <https://hivetests.ethdevops.io>
- Engine API integration and rpc tests are on <https://hivetests2.ethdevops.io>

**To read more about hive, please check [the documentation][doc].**

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

### Contributions

This project takes a different approach to code contributions than your usual FOSS project
with well ingrained maintainers and relatively few external contributors. It is an
experiment. Whether it will work out or not is for the future to decide.

We follow the [Collective Code Construction Contract (C4)][c4], code contribution model,
as expanded and explained in [The ZeroMQ Process][zmq-process]. The core idea being that
any patch that successfully solves an issue (bug/feature) and doesn't break any existing
code/contracts must be optimistically merged by maintainers. Followup patches may be used
for additional polishes – and patches may even be outright reverted if they turn out to
have a negative impact – but no change must be rejected based on personal values.

### License

The hive project is licensed under the [GNU General Public License v3.0][gpl]. You can
find it in the COPYING file.

[doc]: ./docs/overview.md
[c4]: http://rfc.zeromq.org/spec:22/C4/
[zmq-process]: https://hintjens.gitbooks.io/social-architecture/content/chapter4.html
[gpl]: http://www.gnu.org/licenses/gpl-3.0.en.html
