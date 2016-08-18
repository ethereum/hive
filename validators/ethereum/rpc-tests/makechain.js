// This is a small script to generate the RLP encoded blockchain dump for tested
// clients to import before running the RPC tests against.

var fs = require('fs');

// Load and parse the test configuration
var test = require("/rpc-tests/lib/tests/BlockchainTests/bcRPC_API_Test.json");

// Iterate over all the blocks and export them
fs.mkdirSync("/blocks")

var blocks = test.RPC_API_Test.blocks;
for (var i = 0; i < blocks.length; i++) {
	fs.writeFileSync("/blocks/" + i, new Buffer(blocks[i].rlp.substring(2), "hex"));
}
