#!/usr/bin/env python
import os,sys
import binascii
import json
import requests
from web3 import Web3, RPCProvider
import traceback
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

        print("\n#  %s\n" % self )
        print("Success: %d / Fail: %d / Skipped: %d\n" % (len(success), len(failed), len(skipped)))

        def x(l,title):
            if len(l) > 0:
                print("\n## %s\n" % title)
                for test in l:
                    print("* %s" % test.getReport())


        x(failed , "Failed")
        x(skipped, "Skipped")
        x(success, "Successfull")

    def __str__(self):
        return "File `%s`" % self.filename



class Testcase(object):

    def __init__(self,name, jsondata):
        self.name = name
        self.data = jsondata
        self.raw_genesis = None
        self._skipped = True
        self._message = []

    def __str__(self):
        return self.name

    def validate(self):
        required_keys = ["pre","blocks","postState","genesisBlockHeader"]
        missing_keys = []
        for k in required_keys:
            if k not in self.data.keys():
                missing_keys.append(k)

        
        return (len(missing_keys) == 0 ,"Missing keys: %s" % (",".join(missing_keys))) 

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


        if key in self.data['poststate']:
            return self.data['postState'][key]

        return None


    def blocks(self):
        return self.data['blocks']

    def keys(self):
        return None

    def chain(self):

        return None

    def fail(self, message):
        """Set if this test failed"""
        self._success = False
        self._message = message
        self._skipped = False
    
    def success(self, message = []):
        self._success = True
        self._message = message
        self._skipped = False

    def skipped(self, message = []):
        self._skipped = True
        self._message = message


    def wasSuccessfull(self):
        return bool(self._success)

    def wasSkipped(self):
        return self._skipped


    def report(self):
        if self.wasSuccessfull():
            print("%s: Success" % self.name)
            return

        if self.wasSkipped():
            print("%s: Skipped")
        else:
            print("%s: Failed" % self.name)
        
        for msg in self._message:
            print("  %s" % msg)

    def getReport(self):
        outp = ["%s (%s)" % (self.name, self.status())]

        if self._message is not None:
            for msg in self._message:
                outp.append("   * %s" % str(msg))
        return "\n".join(outp)

    def status(self):

        if self._skipped:
            return "skipped"
        if self._success:
            return "success"

        return "failed"

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
            raise Exception("Failed to GET req (%d)" % req.status_code)
        return req.text
    
    def _post(self,path, params = None):
        req = requests.post("%s%s" % (self.hive_simulator , path),  params=params)
        
        if req.status_code != 200:
            raise Exception("Failed to POST req (%d)" % req.status_code)

        return req.text

    def _delete(self,path, params = None):
        req = requests.delete("%s%s" % (self.hive_simulator , path),  params=params)
        
        if req.status_code != 200:
            raise Exception("Failed to DELETE req (%d)" % req.status_code)

        return req.text

    def log(self,msg):
        requests.post("%s/logs" % (self.hive_simulator ), data = msg) 

    def debugp(self, msg):
        self.log(msg)
#        print(msg)


    def blockTests(self, start = 0, end = 1000000000000000000):

        count = 0

        for testfile in getFiles("./tests/BlockchainTests"):
            count = count +1
            if count < start:
                continue
            if count >= end:
                break

            tf = Testfile(testfile)
            self.log("Commencing testfile [%d] (%s)\n " % (count, tf))
            for testcase in tf.tests() :
                (ok, err) = testcase.validate()

                if ok:
                    self.executeBlocktest(testcase)
                else:
                    self.log("Skipped test %s" % testcase )
                    testcase.skipped(["Testcase failed initial validation", err])

                self.log("Test: %s %s (%s)" % (testfile, testcase, testcase.status()))

                testcase.report()
                #break

            tf.report()
            count = count +1
            


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
#                   outf.write(binary_string)



        return (g_file, c_file, b_folder)



    def executeBlocktest(self,testcase):
        genesis = None
        init_chain = None
        blocks = None

        try:
            (genesis, init_chain, blocks ) = self.generateArtefacts(testcase)
        except Exception, e:
            traceback.print_exc(file=sys.stdout)
            testcase.fail(["Failed to write test data to disk", str(e)])
            return False
        #HIVE_INIT_GENESIS path to the genesis file to seed the client with (default = "/genesis.json")
        #HIVE_INIT_CHAIN path to an initial blockchain to seed the client with (default = "/chain.rlp")
        #HIVE_INIT_BLOCKS path to a folder of blocks to import after seeding (default = "/blocks/")
        #HIVE_INIT_KEYS path to a folder of account keys to import after init (default = "/keys/")

        #HIVE_FORK_HOMESTEAD
        params = {
            "HIVE_INIT_GENESIS": genesis, 
            "HIVE_INIT_BLOCKS" : blocks,
            # These tests run with Frontier rules
            "HIVE_FORK_HOMESTEAD" : "20000",
            "HIVE_FORK_TANGERINE" : "20000",
            "HIVE_FORK_SPURIOUS"  : "20000",
#             "HIVE_INIT_CHAIN" : chain,
        }
        node = None
        try:
            node = self.newNode(params)
        except Exception, e:
            testcase.fail(["Failed to start node (%s)" % str(e)])
            return False


        #self.log("Started node %s" % node)

        try:
            (ok, err ) = self.verifyPreconditions(testcase, node)

            if not ok:

                testcase.fail(["Preconditions failed",err])
                return False

            (ok, err) = self.verifyPostconditions(testcase, node)
            self.debugp("verifyPostconditions returned %s" % ok)

            if not ok: 
                testcase.fail(["Postcondition check failed",err])
                return False

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
                return "Found `%s`, expected `%s`"  % (v, exp)
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
                return "Found `%s`, expected `%s`"  % (v, exp)
            return None

        def _verifyEqHex(v,exp):
            if canonicalize(v) != canonicalize(exp):
                return "Found `%s`, expected `%s`"  % (v, exp)
            return None

        for address, poststate_account in testcase.postconditions().items():
            
            # Keep track of what we check, so we don't miss any postconditions

            checked_conditions = set()
            should_check = set(poststate_account.keys())
            # Actual values
            _n = None
            _c = None
            _b = None
            # Parity fails otherwise...
            if address[:2] != "0x":
                address = "0x%s" % address

            try:
                _n = node.web3.eth.getTransactionCount(address)
                _c = node.web3.eth.getCode(address)
                _b = node.web3.eth.getBalance(address)
            except Exception, e:
                errs.append("Postcondition verification failed %s" % str(e))
                return (False, errs)


            # Expected values

            if 'nonce' in poststate_account:
                #schecked_conditions.add('nonce')
                exp = hex2big(poststate_account["nonce"])
                err = _verifyEqRaw(_n, exp)
 #               self.debugp("Postcond check nonce %s = %s => %s" % (_n, exp, err)
                if err is not None:
                    errs.append("Nonce error (%s)" % address)
                    errs.append(err)

            if 'code' in poststate_account:
                checked_conditions.add('code')
                exp = poststate_account["code"]
                err = _verifyEqRaw(_c, exp)
#                self.debugp("Postcond check code %s = %s => %s" % (_c, exp, err)
                if err is not None:
                    errs.append("Code error (%s)" % address)
                    errs.append(err)


            if 'balance' in poststate_account:
                checked_conditions.add('balance')
                exp = hex2big(poststate_account["balance"])
                err = _verifyEqRaw(_b, exp)
 #               self.debugp("Postcond check balance %s = %s => %s" % (_b, exp,err)
                if err is not None:
                    errs.append("Balance error (%s)" % address)
                    errs.append(err)

            if 'storage' in poststate_account:
                checked_conditions.add('storage')
                # Must iterate over storage
                self.debugp("Postcond check balance")
                for _hash,exp in poststate_account['storage'].items():
                    value = node.web3.eth.getState(address, _hash )
                    err = _verifyEqHex(value, exp)
                    if err is not None:
                        errs.append("Storage error (%s) key %s" % (address, _hash))
                        errs.append(err)

            missing_checks = should_check.difference(checked_conditions)

            if len(missing_checks) > 0:
                self.log("Error: Missing postcond checks: %s" % ",".join(missing_checks))

        return (len(errs) == 0, errs)

    def newNode(self, params):
        _id = self._post("/nodes", params)
        _ip = self._get("/nodes/%s" % _id)
        return HiveNode(_id, _ip)

    def killNode(self,node):
        self._delete("/nodes/%s" % node.nodeId)


# Model for the Hive interaction

class FakeEth():
    def getTransactionCount(arg):
        return 10000
    def getBalance(arg):
        return 1000
    def getCode(arg):
        return "0xDEADBEEF"
    def getBlock(arg,arg2):
        return {u'hash':"0x0000", u'stateRoot':"0x0102030405060708"}

class FakeWeb3():
    def __init__(self):
        self.eth = FakeEth()

class HiveTestNode(HiveNode):

    def __init__(self, nodeId = None, nodeIp = None):
        self.nodeId ="Testnode"
        self.web3 = FakeWeb3()


    def invokeRPC(self,method, arguments):
        """ Can be used to call things not implemented in web3. 
        Example:         
            invokeRPC("debug_traceTransaction", [txHash, traceOpts]))
        """
        return self.web3._requestManager.request_blocking(method, arguments)



    def __str__(self):
        return "Node[%s]@%s"%(self.nodeId, self.ip)

class HiveTestAPI(HiveAPI):

    def __init__(self):
        super(HiveAPI, self).__init__()

    def newNode(self, params):
        return HiveTestNode()

    def killNode(self, node):
        pass
    def generateArtefacts(self,testcase):
        return (None, None, None)

    def log(self,msg):
        print("LOG: %s" % msg)

def test():
    hive = HiveTestAPI()
    hive.blockTests()

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

    hive.blockTests(start = 0, end=2)

if __name__ == '__main__':
    main(sys.argv[1:])
    #test()
