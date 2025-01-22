# Running Hive Beacon-Sync Tests

The beacon sync suite of tests requires access to an external consensus provider to help bootstrap the network. To run these tests you must pass in the following build arguments:

- `--sim.buildarg PORTAL_CONSENSUS_URL=<url>`: The URL of the consensus provider to use. This can be a local or remote URL.
- `--sim.buildarg PORTAL_CONSENSUS_AUTH=<id>:<secret>`: If you're using a pandaops provider, you must pass in your ID and secret to authenticate with the service.
