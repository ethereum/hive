# Optimism P2P test suite

This test suite tests the P2P protocol between the sequencer and verifier on
the optimism network.

    hive --sim optimism/p2p --client=op-l1,op-l2,op-proposer,op-batcher,op-sequencer,op-verifier --docker.output
