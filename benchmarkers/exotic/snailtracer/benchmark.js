var Web3 = require('web3');
var web3 = new Web3(new Web3.providers.HttpProvider('http://' + process.env.HIVE_CLIENT_IP + ':8545'));

var SnailTracer = web3.eth.contract([{"constant":true,"inputs":[],"name":"Benchmark","outputs":[{"name":"r","type":"bytes1"},{"name":"g","type":"bytes1"},{"name":"b","type":"bytes1"}],"type":"function"}]);
var snailTracer = SnailTracer.at("0x0bcc3ffef5ba396576b881445976a9c7bcd4bf36");

for (var i = 0; i < process.env.HIVE_BENCHMARKER_ITERS; i++) {
	snailTracer.Benchmark({gas: 1000000000});
}
