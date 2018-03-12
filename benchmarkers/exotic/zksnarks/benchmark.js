var Web3 = require('web3');
var web3 = new Web3(new Web3.providers.HttpProvider('http://' + process.env.HIVE_CLIENT_IP + ':8545'));

var ZkSnarks = web3.eth.contract([{"constant":true,"inputs":[],"name":"verifyTx","outputs":[{"name":"r","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"}]);
var zksnarks = ZkSnarks.at("0xad5518f4dba069b02097e8201b2c49e487de3655");

for (var i = 0; i < process.env.HIVE_BENCHMARKER_ITERS; i++) {
	if (!zksnarks.verifyTx({gas: 2000000})) {
		throw "failed to verify snark";
	};
}
