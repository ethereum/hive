#!/usr/bin/env python
import os,sys
import binascii
import json
import requests
from web3 import Web3, RPCProvider

from collections import defaultdict

class HiveNode(object):

    def __init__(self, nodeId, nodeIp):
        self.nodeId =nodeId
        self.ip = nodeIp
        self.web3 = Web3(RPCProvider(host=self.ip, port="8545"))

    

    def __str__(self):
        return "Node[%s]@%s"%(self.nodeId, self.ip)



class Testfile(object):

    def __init__(self,fname):
        self.filename = fname

    def tests(self):
        with open(self.filename,"r") as infile: 
            json_data = json.load(infile)
            for k,v in json_data.items():
                yield Testcase(k,v)

    def __str__(self):
        return self.filename

class Testcase(object):

    def __init__(self,name, jsondata):
        self.name = name
        self.data = jsondata

    def __str__(self):
        return self.name

    def validate(self):
        required_keys = ["pre","blocks","genesisRLP","postState","genesisBlockHeader"]
        missing_keys = []
        for k in required_keys:
            if k not in self.data.keys():
                missing_keys.append(k)
                print("Missing key %s" % k)
        return len(missing_keys) == 0

    def genesis(self):
        # Genesis block
        raw_genesis = self.data['genesisBlockHeader']
        # The pre-section

#                "pre" : {
#            "a94f5374fce5edbc8e2a8697c15331677e6ebf0b" : {
#                "balance" : "0x09184e72a000",
#                "code" : "0x",
#                "nonce" : "0x00",
#                "storage" : {
#                }
#            }
        #alloc = {}
        #for address, acct in self.data['pre'].items()
        #    alloc[address]
        raw_genesis['alloc'] = self.data['pre']
        return raw_genesis


    def blocks(self):
        return self.data['blocks']

    def keys(self):
        return None

    def chain(self):

        return None

def getFiles(root):
    print("Root %s" % root)
    counter = 0
    for subdir, dirs, files in os.walk(root):
        print("subdir %s" % subdir)
        for fil in files:
            filepath = subdir + os.sep + fil
            if filepath.endswith(".json"):
                yield filepath 
                counter = counter +1
            if counter == 10:
                return

class HiveAPI(object):
    
    def __init__(self, hive_simulator):
        self.nodes = []
        self.hive_simulator = hive_simulator

    def _get(self,path, params = None):
        req = requests.get("%s%s" % (self.hive_simulator , path),  params=params)
        if req.status_code != 200:
            print(req.text)
            raise Exception("Failed to GET req (%d)" % req.status_code)
        return req.text
    
    def _post(self,path, params = None):
        req = requests.post("%s%s" % (self.hive_simulator , path),  params=params)
        
        if req.status_code != 200:
            print("!! Error starting client")
            print(req.text)
            print("--------------------")
            raise Exception("Failed to POST req (%d)" % req.status_code)
        return req.text


    def blockTests(self):
        for testfile in getFiles("./tests/BlockchainTests"):
            tf = Testfile(testfile)
            print("Testfile %s" % tf)
            for testcase in tf.tests() :
                print("Test: %s" % testcase)
                if testcase.validate():
                    self.executeBlocktest(testcase)

    def generateArtefacts(self,testcase):
        try:
           os.makedirs("./artefacts/%s/" % testcase)
           os.makedirs("./artefacts/%s/blocks/" % testcase)
           os.makedirs("./artefacts/%s/" % testcase)
        except Exception, e:
            pass

        g_file = "./artefacts/%s/genesis.json" % testcase
        c_file = "./artefacts/%s/chain.rlp" % testcase
        b_folder = "./artefacts/%s/blocks/" % testcase

        if testcase.genesis() is not None:
            with open(g_file,"w+") as g:
                json.dump(testcase.genesis(),g)
#        if testcase.chain() is not None:
#            with open(c_file,"w+") as g:
#                json.dump(testcase.chain(),g)
        if testcase.blocks() is not None:
            for block in testcase.blocks():
                counter = 1
                b_file = "./artefacts/%s/blocks/%d.rlp" % (testcase, int(block['blocknumber']))
                binary_string = binascii.unhexlify(block['rlp'][2:])
                with open(b_file,"wb+") as outf:
                    outf.write(binary_string)

        return (g_file, c_file, b_folder)

    def executeBlocktest(self,testcase):


        (genesis, init_chain, blocks ) = self.generateArtefacts(testcase)


        print("Running test %s" % testcase)

        #HIVE_INIT_GENESIS path to the genesis file to seed the client with (default = "/genesis.json")
        #HIVE_INIT_CHAIN path to an initial blockchain to seed the client with (default = "/chain.rlp")
        #HIVE_INIT_BLOCKS path to a folder of blocks to import after seeding (default = "/blocks/")
        #HIVE_INIT_KEYS path to a folder of account keys to import after init (default = "/keys/")
        params = {
            "HIVE_INIT_GENESIS" : genesis, 
           # "HIVE_INIT_BLOCKS" : blocks,
#             "HIVE_INIT_CHAIN" : chain,
        }

        node = self.newNode(params)

        print("Started node %s" % node)
        import time
        print("Sleeping 10 seconds")
        time.sleep(10)
        print("Latest block:")
        latest =  node.web3.eth.getBlock("latest")
        print(latest)
        print("Hash: %s"  % latest[u'hash'])
        print("Should be ")
        print(testcase.genesis()[u'hash'])


        print("Balance ")
        print node.web3.eth.getBalance("0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")

        print("Code ")
        print node.web3.eth.getCode("0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")

        print("Empty Code:")
        print node.web3.eth.getCode("0x1111111111122222222222333333444444444455")
        
        print("Nonce: ")
        print node.web3.eth.getTransactionCount("0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")

        print("Quitting...")
        sys.exit(1)

    def newNode(self, params):

        count = 0
        while count < 10:
            count = count +1
            try:
                _id = self._post("/nodes", params)
                _ip = self._get("/nodes/%s" % _id)
                return HiveNode(_id, _ip)
            except Exception, e:
                if count == 10:
                    raise e





def main(args):
    print("Validator started\n")
    print("-" * 40)
    if 'HIVE_SIMULATOR' in os.environ:
        hivesim = os.environ['HIVE_SIMULATOR']
    elif len(args) > 0:
        hivesim = args[0]
    else:
        hivesim = "http://127.0.0.1" 

    print("Hive simulator: %s\n" % hivesim)
    hive = HiveAPI(hivesim)

    hive.blockTests()


#    c = hive.newNode()
#    print("Got client %s\n" % c)
#
#    client =c.ethClient()
#
#    print(client.make_request("personal_importRawKey",[
#        "0000000000000000000000000000000000000001", "pass"]))
#    coinbase = client.get_coinbase()
#
#    print(client.make_request("personal_listAccounts",[]))
#
#    # Fill up pending
#    for i in range(0,60):
#        client.make_request("personal_signAndSendTransaction",[
#                {
#                    "from":coinbase, 
#                    "to":"0x0000000000000000000000000000000000000000", 
#                    "value":1, 
#                },
#                "pass"      
#            ])
#        if i % 100 == 0:
#            print(".\n")
#    #Fill up future
#
#
#    txpool =  client.make_request("txpool_content",[])
#    pending = json.loads(txpool)['result'].pending
#    queued = json.loads(txpool)['result'].pending
#    print("Pending %d, Queued %d\n" , len(pending), len(queued))


if __name__ == '__main__':
    main(sys.argv[1:])

