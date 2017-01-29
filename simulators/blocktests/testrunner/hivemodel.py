import requests
from web3 import Web3, RPCProvider
import traceback,sys,os
import binascii
import json
from testmodel import Testcase, Testfile
from utils import canonicalize, getFiles, hex2big
import time

class ReportCollector(object):
    

    def __init__(self):
        self.data = { "meta" : [], "results" : []}


    def addResult(self,obj):
        self.data['results'].append(obj)

    def addMeta(self, k,v):
        self.data['meta'].append([k,v])

    def getResults(self):
        self.addMeta("Execution end", time.asctime( time.localtime(time.time())))
        return json.dumps(self.data)

reporter = ReportCollector()

# Model for the Hive interaction
class HiveNode(object):

    def __init__(self, nodeId, nodeIp, clienttype="N/A"):
        self.nodeId =nodeId
        self.ip = nodeIp
        self.clienttype = clienttype
        self.web3 = Web3(RPCProvider(host=self.ip, port="8545"))


    def invokeRPC(self,method, arguments):
        """ Can be used to call things not implemented in web3. 
        Example:         
            invokeRPC("debug_traceTransaction", [txHash, traceOpts]))
        """
        return self.web3._requestManager.request_blocking(method, arguments)



    def __str__(self):
        return "Node[%s]@%s" % (self.nodeId, self.ip)

class HiveAPI(object):
    
    def __init__(self, hive_simulator):
        self.nodes = []
        self.hive_simulator = hive_simulator
        self.clienttype = self._get("/clientinfo")
        self.log("HiveAPI using %s" % self.clienttype)


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

    def sendReport(self, fname, data):
        requests.post("%s/report/%s" % (self.hive_simulator,fname), data = data)

    def blockTests(self, start = 0, end = 1000000000000000000, whitelist = [], blacklist =[]):

        count = 0

#        for testfile in getFiles("./tests/BlockchainTests", limit=10):
        for testfile in getFiles("./tests/BlockchainTests"):
            count = count +1
            if count < start:
                continue
            if count >= end:
                break

            tf = Testfile(testfile)
            self.log("Commencing testfile [%d] (%s)\n " % (count, tf))
            for testcase in tf.tests() :

                if len(whitelist) > 0 and str(testcase) not in whitelist:
                    testcase.skipped(["Testcase not in whitelist"])
                    continue

                if len(blacklist) > 0 and str(testcase) in blacklist:
                    testcase.skipped(["Testcase in blacklist"])
                    continue

                (ok, err) = testcase.validate()

                if ok:
                    self.executeBlocktest(testcase)
                else:
                    self.log("Skipped test %s" % testcase )
                    testcase.skipped(["Testcase failed initial validation", err])

                self.log("Test: %s %s (%s)" % (testfile, testcase, testcase.status()))
                reporter.addResult({
                    "testname"  : testcase.name, 
                    "filename"  : tf.name, 
                    "client"    : self.clienttype,
                    "status"    : testcase.status(),
                    "message"   : testcase._message,
                    "instance"  : testcase.nodeInstance,
                    })

                #testcase.report()
                break

#            r = tf.getReport(self.clienttype)
#            self.sendReport("%s.md" % self.clienttype.replace(":","-"),r)
            count = count +1
        
        self.sendReport("data.jsonp", "onData(%s);" % reporter.getResults())
            

    def generateArtefacts(self,testcase):
        try:
           os.makedirs("./artefacts/%s/" % testcase)
           os.makedirs("./artefacts/%s/blocks/" % testcase)
           os.makedirs("./artefacts/%s/" % testcase)
        except Exception, e:
            #print("Exception creating directory:%s " % e)
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

        }
        node = None
        try:
            node = self.newNode(params)
        except Exception, e:
            testcase.fail(["Failed to start node (%s)" % str(e)])
            return False


        self.log("Started node %s" % node)

        try:
            testcase.setNodeInstance(node.nodeId)
            testcase.setClientType(node.clienttype)
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
                if err is not None:
                    errs.append("Nonce error (%s)" % address)
                    errs.append(err)

            if 'code' in poststate_account:
                checked_conditions.add('code')
                exp = poststate_account["code"]
                err = _verifyEqRaw(_c, exp)
                if err is not None:
                    errs.append("Code error (%s)" % address)
                    errs.append(err)


            if 'balance' in poststate_account:
                checked_conditions.add('balance')
                exp = hex2big(poststate_account["balance"])
                err = _verifyEqRaw(_b, exp)
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
        return HiveNode(_id, _ip, self.clienttype)

    def killNode(self,node):
        self._delete("/nodes/%s" % node.nodeId)
