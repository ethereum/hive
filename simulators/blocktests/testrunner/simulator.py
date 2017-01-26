#!/usr/bin/env python
import os,sys
import binascii
import json
import requests
from web3 import Web3, RPCProvider

from collections import defaultdict

# Some utilities

def canonicalize(v):
    if type(v) == str or type(v) == unicode:
        v = v.lower()
        if v.startswith("0x"):
            return str(v[2:])
    return v

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

def hex2big(txt):
    txt = canonicalize(txt)
    print("txt: %s (%s)" % (str(txt), type(txt)))
    return int(txt,16)

# Model for the testcases

class Testfile(object):

    def __init__(self,fname):
        self.filename = fname
        self._tests = []

    def tests(self):
        with open(self.filename,"r") as infile: 
            json_data = json.load(infile)
            for k,v in json_data.items():
                t = Testcase(k,v)
                self._tests.append(t)
                yield t

    def report(self):
        skipped = []
        failed = []
        success = []
        for test in self._tests:
            if test.wasSkipped(): 
                skipped.append(test)
            elif not test.wasSuccessfull():
                failed.append(test)
            else:
                success.append(test)

        print("#  %s\n" % self.filename )
        print("Success: %d / Fail: %d / Skipped: %d\n" % (len(success), len(failed), len(skipped)))

        def x(l,title):
            if len(l) > 0:
                print("## %s\n" % title)
                for test in l:
                    print("  * %s" % test)

        x(failed , "Failed")
        x(skipped, "Skipped")
        x(success, "Successfull")

    def __str__(self):
        return self.filename



class Testcase(object):

    def __init__(self,name, jsondata):
        self.name = name
        self.data = jsondata
        self.raw_genesis = None
        self._skipped = True
        self._message = ""
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

    def genesis(self, key = None):
        # Genesis block
        if self.raw_genesis is None:
            raw_genesis = self.data['genesisBlockHeader']

            # Turns out the testcases have noncewritten as 0102030405060708. 
            # Which is supposed to be interpreted as 0x0102030405060708. 
            # But if it's written as 0102030405060708 in the genesis file, 
            # it's interpreted differently. So we'll need to mod that on the fly 
            # for every testcase.
            nonce = raw_genesis[u'nonce']
            if not raw_genesis[u'nonce'][:2] == '0x':
                raw_genesis[u'nonce'] = '0x'+raw_genesis[u'nonce']

            raw_genesis['alloc'] = self.data['pre']
            self.raw_genesis = raw_genesis

        if key is None:
            return self.raw_genesis

        return self.raw_genesis[key]

    def postconditions(self, key = None):
        postconditions = self.data['postState']

        if key is None:
            return self.data['postState']


        if self.has_postcondition(key):
            return self.data['postState'][key]

        return None
    def has_postcondition(self, key):
        return self.data['postState'].has_key(key)

    def blocks(self):
        return self.data['blocks']

    def keys(self):
        return None

    def chain(self):

        return None

    def fail(self, message):
        """Set if this test failed"""
        self._success = True
        self._message = message
        self._skipped = False
    
    def success(self, message = ""):
        self._success = True
        self._message = message
        self._skipped = False

    def skipped(self, message = ""):
        self._skipped = True
        self._message = message


    def wasSuccessfull(self):
        return bool(self._success)

    def wasSkipped(self):
        return self._skipped

    def report(self):
        if self.wasSkipped():
            print("%s: Skipped")
            for msg in self.msg:
                print("  %s" % msg)
            return

        if self.wasSuccessfull():
            print("%s: Ok." % self.name)
            return

        print("%s: Failed" % self.name)
        for msg in self.msg:
            print("  %s" % msg)


# Model for the Hive interaction
class HiveNode(object):

    def __init__(self, nodeId, nodeIp):
        self.nodeId =nodeId
        self.ip = nodeIp
        self.web3 = Web3(RPCProvider(host=self.ip, port="8545"))


    def invokeRPC(self,method, arguments):
        """ Can be used to call things not implemented in web3. 
        Example:         
            invokeRPC("debug_traceTransaction", [txHash, traceOpts]))
        """
        return self.web3._requestManager.request_blocking(method, arguments)



    def __str__(self):
        return "Node[%s]@%s"%(self.nodeId, self.ip)

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

    def _delete(self,path, params = None):
        req = requests.delete("%s%s" % (self.hive_simulator , path),  params=params)
        
        if req.status_code != 200:
            print("!! Error killing client")
            print(req.text)
            print("--------------------")
            raise Exception("Failed to DELETE req (%d)" % req.status_code)

        return req.text

    def log(self,msg):
        requests.post("%s/logs" % (self.hive_simulator ), data = msg) 


    def blockTests(self):
        for testfile in getFiles("./tests/BlockchainTests"):
            tf = Testfile(testfile)
            print("Testfile %s" % tf)
            for testcase in tf.tests() :
                print(" Test: %s" % testcase)
                if testcase.validate():
                    self.executeBlocktest(testcase)
                else:
                    print("Skipped test %s" % testcase )
                    testcase.skipped("Testcase failed initial validation")

                testcase.report()
            tf.report()
            return


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

        if testcase.blocks() is not None:
            counter = 1
            for block in testcase.blocks():
                
                b_file = "./artefacts/%s/blocks/%04d.rlp" % (testcase, counter)
                binary_string = binascii.unhexlify(block['rlp'][2:])
                with open(b_file,"wb+") as outf:
                    outf.write(binary_string)
                counter = counter +1

# Maybe do it all in one go: 
#        if testcase.blocks() is not None:
#            b_file = "./artefacts/%s/blocks/blocks.rlp" % (testcase)
#            with open(b_file,"wb+") as outf:
#                for block in testcase.blocks():
#                    binary_string = binascii.unhexlify(block['rlp'][2:])
#                    outf.write(binary_string)
#


        return (g_file, c_file, b_folder)



    def executeBlocktest(self,testcase):
        (genesis, init_chain, blocks ) = self.generateArtefacts(testcase)

        #HIVE_INIT_GENESIS path to the genesis file to seed the client with (default = "/genesis.json")
        #HIVE_INIT_CHAIN path to an initial blockchain to seed the client with (default = "/chain.rlp")
        #HIVE_INIT_BLOCKS path to a folder of blocks to import after seeding (default = "/blocks/")
        #HIVE_INIT_KEYS path to a folder of account keys to import after init (default = "/keys/")
        params = {
            "HIVE_INIT_GENESIS": genesis, 
            "HIVE_INIT_BLOCKS" : blocks,
#             "HIVE_INIT_CHAIN" : chain,
        }

        node = self.newNode(params)

        self.log("Started node %s" % node)

        try:
            (ok, err ) = self.verifyPreconditions(testcase, node)

            if not ok:
                testcase.failed("Preconditions failed", err)
                return False

            (ok, err) = self.verifyPostconditions(testcase, node)

            if not ok: 
                testcase.failed("Postcondition check failed", err)

            testcase.success()
            return True

        finally:
            self.killNode(node)



    def verifyPreconditions(self, testcase, node):
        """ Verify preconditions 
        @return (bool isOk, list of error messags) 
        """

        first =  node.web3.eth.getBlock(0)
        errs = []

        def _verifyEq(v,exp):
            v = canonicalize(v)
            exp = canonicalize(exp)
            if v != exp:
                return "Found %s, expected %s"  % (v, exp)
            return None

        err = _verifyEq(first[u'hash'], testcase.genesis('hash'))

              # Check hash
        if err is not None:
            errs.append("Hash error")
            errs.append(err)

            
        # Check stateroot (only needed if hash failed really...)
        state_err = _verifyEq(first[u'stateRoot'],testcase.genesis('stateRoot'))
        if state_err is not None:
            errs.append("State differs")
            errs.append(state_err)
        
        return (len(errs) == 0, errs)


    def verifyPostconditions(self, testcase, node):
        """ Verify postconditions 
        @return (bool isOk, list of error messags) 
        """
        errs = []
        def _verifyEqRaw(v,exp):
            if v != exp:
                return "Found %s, expected %s"  % (v, exp)
            return None

        def _verifyEqHex(v,exp):
            if canonicalize(v) != canonicalize(exp):
                return "Found %s, expected %s"  % (v, exp)
            return True


        for address, account in testcase.postconditions().items():
            # Actual values
            _n = node.web3.eth.getTransactionCount(address)
            _c = node.web3.eth.getCode(address)
            _b = node.web3.eth.getBalance(address)
            # Expected values

            if testcase.has_postcondition("nonce"):
                exp = hex2big(testcase.postconditions("nonce"))
                err = _verifyEqRaw(_n, exp)
                if err is not None:
                    errs.append("Nonce error (%s)" % address)
                    errs.append(err)

            if testcase.has_postcondition("code"):
                exp = testcase.postconditions("code")
                err = _verifyEqRaw(_n, exp)
                if err is not None:
                    errs.append("Code error (%s)" % address)
                    errs.append(err)

            if testcase.has_postcondition("balance"):
                exp = hex2big(testcase.postconditions("balance"))
                err = _verifyEqRaw(_n, exp)
                if err is not None:
                    errs.append("Balance error (%s)" % address)
                    errs.append(err)

            if testcase.has_postcondition("storage"):
                # Must iterate over storage
                for _hash,exp in account[key].items():
                    value = node.web3.eth.getState(address, _hash )

                    err = _verifyEqHex(value, exp)
                    if err is not None:
                        errs.append("Storage error (%s) key %s" % (address, _hash))
                        errs.append(err)

        return (len(errs) == 0, errs)

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

    def killNode(self,node):
        self._delete("/nodes/%s" % node.nodeId)



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

