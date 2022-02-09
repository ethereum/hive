# devp2p test suites

This simulator runs devp2p protocol tests. At this time, there are tests for 'eth' and
'discv4'.

The simulator runs all available tests by default.

    hive --sim devp2p

You can select individual suites using hive's `--sim.limit` flag:

    hive --sim devp2p --sim.limit eth
