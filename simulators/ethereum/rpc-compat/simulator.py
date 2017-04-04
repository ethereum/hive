# inspired by Hive simulator for consensus tests
# https://github.com/karalabe/hive/blob/243b64b80a84c131b462ef0ad380887c2c2c2f09/simulators/ethereum/consensus/simulator.py
# https://github.com/karalabe/hive/blob/243b64b80a84c131b462ef0ad380887c2c2c2f09/simulators/ethereum/consensus/hivemodel.py

import os
import sys
import json
import binascii
import requests


# TODO: test runners should live with the hive simulator, not copied from interfaces/rpc-specs-tests
from test_rpc_methods import run_test
from validate_tests import validate_tests


class HiveReporter(object):

    def __init__(self, nodeUrl):
        self.session = requests.Session()
        self.nodeUrl = nodeUrl

    def subresult(self, name, success, errormsg, details = None):
        params = {
                "name" : name,
                "success" : success
        }
        if errormsg is not None:
            params["error"] = errormsg

        data = None
        if details is not None:
            data = {"details" : json.dumps(details) }

        post_response = self.session.post(self.nodeUrl + "/subresults", params = params, data = data)
        #print("subresult post_response:", post_response)
        return


print("RPEECEE SIM STARTED")

hivesim = os.environ['HIVE_SIMULATOR']
print("Hive simulator: {}\n".format(hivesim))

session = requests.Session()


try:
    os.makedirs('/blocks')
except OSError as exception:
    if exception.errno != errno.EEXIST:
        raise

with open('/bcRPC_API_Test.json') as block_data:
    blocks_json = json.load(block_data)

block_i = 0
for block in blocks_json['RPC_API_Test']['blocks']:
    binary_string = binascii.unhexlify(block['rlp'][2:])
    block_file = '/blocks/' + str(block_i)
    with open(block_file,"wb+") as outf:
        outf.write(binary_string)
    block_i = block_i + 1

params = {
    "HIVE_INIT_GENESIS": '/genesis.json',
    "HIVE_INIT_BLOCKS" : '/blocks',
    "HIVE_FORK_HOMESTEAD": 1150000,
    "HIVE_FORK_TANGERINE": 20000,
    "HIVE_FORK_SPURIOUS": 20000
}

# initialize new node
req = session.post(hivesim + '/nodes', params=params)
print("req: {}".format(req))
print("req.status_code: {}".format(req.status_code))
print("req.text: {}".format(req.text))
node_id = req.text


req2 = session.get(hivesim + '/nodes/' + node_id,  params=params)
print("req2: {}".format(req2))
print("req2.status_code: {}".format(req2.status_code))
print("req2.text: {}".format(req2.text))
node_ip = req2.text

print("node_ip: {}".format(node_ip))
node_url = 'http://' + node_ip + ":8545"


try:
    # test basic rpc use
    payload = {"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}
    res = session.post(node_url, json=payload, timeout=10)
    print("clientVersion res.json(): {}".format(res.json()))

    # unlock account
    payload = {"jsonrpc":"2.0","method":"personal_unlockAccount","params":["0xbe93f9bacbcffc8ee6663f2647917ed7a20a57bb", "password", 300],"id":1}
    print("sending unlock_account rpc payload:", payload)
    res = session.post(node_url, json=payload, timeout=10)
    print("unlock_account res.json(): {}".format(res.json()))


except Exception as e:
    print("got exception: {}".format(e))
    print("got exception str: {}".format(str(e)))


reporter = HiveReporter(hivesim)

print("validating tests and schemas before running them.")
validate_tests()
print("tests validated.")

test_files = os.listdir("./tests")

for test_name in test_files:
   run_test(test_name, session, node_url, reporter)


print("RPEECEE SIM FINISHED")
sys.exit(0)
