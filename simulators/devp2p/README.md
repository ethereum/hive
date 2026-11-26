# devp2p test suites

This simulator runs devp2p protocol tests. At this time, there are tests for 'eth' and
'discv4'.

The simulator runs all available tests by default.

    hive --sim devp2p

You can select individual suites using hive's `--sim.limit` flag:

    hive --sim devp2p --sim.limit eth

The `eth75` suite tests EIP-8437 propagation of EIP-8288 proof wrappers. It starts the
client on the EIP-8288 test chain in go-ethereum's `cmd/devp2p/internal/ethtest/testdata/eip8288`,
whose fork environment sets `HIVE_EIP8288_TIMESTAMP`, and once more with that fork scheduled
in the future. Clients map `HIVE_EIP8288_AGGREGATION_PROBABILITY`, the chance that the node
aggregates a given transaction-hash bucket, where they implement mempool aggregation.
