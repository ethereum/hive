import requests
import traceback,sys,os
import binascii
import json
from testmodel import Testcase, Testfile
from utils import canonicalize, getFiles, hex2big
import time


RULES_FRONTIER = 0
RULES_HOMESTEAD = 1
RULES_TANGERINE = 2
RULES_SPURIOUS = 4

# Model for the Hive interaction
class HiveNode(object):

    def __init__(self, nodeId, nodeIp):
        self.nodeId =nodeId
        self.ip = nodeIp
        self.session = requests.Session()
        self.url = "http://%s:%d" % (self.ip, 8545) 
        self.rpcid = 1


    def _getNodeData(self, method, params):
        payload = {"jsonrpc":"2.0","method":method,"params":params,"id":self.rpcid}
        self.rpcid = self.rpcid+1
        r = self.session.post(self.url,json = payload)
        return r.json()['result']        

    def getBlockByNumber(self,blnum):
        return self._getNodeData("eth_getBlockByNumber", [str(int(blnum)),True])


    def getNonce(self,address):
        
        j = self._getNodeData("eth_getTransactionCount", [address,"latest"])
        return int(j[2:], 16 )


    def getBalance(self,address):

        j = self._getNodeData("eth_getBalance", [address,"latest"])
        return int(j[2:], 16 )


    def getCode(self,address):
        return self._getNodeData("eth_getCode", [address,"latest"])


    def getStorageAt(self,address, _hash):
        return self._getNodeData("eth_getStorageAt", [address,_hash, "latest"])

    def __str__(self):
        return "Node[%s]@%s" % (self.nodeId, self.ip)

class HiveAPI(object):
    
    def __init__(self, hive_simulator):
        self.nodes = []
        self.hive_simulator = hive_simulator

    def _get(self,path, params = None):
        req = requests.get("%s%s" % (self.hive_simulator , path),  params=params)
        if req.status_code != 200:
            raise Exception("Failed to GET req (%d)" % req.status_code)
        return req.text
    
    def _post(self,path, params = None, data = None):
        req = requests.post("%s%s" % (self.hive_simulator , path),  params=params, data = data)
        
        if req.status_code != 200:
            raise Exception("Failed to POST req (%d):%s" % (req.status_code, req.text))

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


    def subresult(self, name, success, errormsg, details = None ):
        params = {
                "name" : name, 
                "success" : success
        }
        if errormsg is not None:
            params["error"] = errormsg

        data = None
        if details is not None:
            data = {"details" : json.dumps(details) }

#        print("Sending the following data with request: %s" % data)

        return self._post("/subresults", params = params, data = data);


#    def generalStateTests(self, start =0 , end = -1, whitelist = [], blacklist = [], testfiles = []):
#        self._performTests(start,end,whitelist, blacklist, testfiles, GeneralStateTestExecutor)
#
    def blockTests(self, start =0 , end = -1, whitelist = [], blacklist = [], testfiles = [],executor= None) :
        return self._performTests(start,end,whitelist, blacklist, testfiles, executor)

    def _performTests(self, start = 0, end = -1, whitelist = [], blacklist =[], testfiles=[],executor= None):

        count = 0
        for testfile in testfiles:

            count = count +1
            if count < start:
                continue
            if 0 <= end <= count:
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
                    executor.executeTestcase(testcase)
                else:
                    self.log("%s failed initial validation" % testcase )
                    testcase.fail(["Testcase failed initial validation", err])

                self.log("Test: %s %s (%s)" % (testfile, testcase, testcase.status()))

                self.subresult(
                        "%s:%s" % (tf.filename, testcase.name),
                        testcase.wasSuccessfull(),
                        testcase.topLevelError(),
                        testcase.details(),
                    )

                break
            
        return True

    def newNode(self, params):
        _id = self._post("/nodes", params)
        _ip = self._get("/nodes/%s" % _id)
        return HiveNode(_id, _ip)

    def killNode(self,node):
        self._delete("/nodes/%s" % node.nodeId)

class TestExecutor(object):
    """ Test-execution engine

    This should probably be moved into 'testmodel' instead. 

    """
    def __init__(self, hiveapi, rules = RULES_FRONTIER):
        self.hive = hiveapi
        self.rules = rules

    def log(msg):
        self.hive.log(msg)

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

        return (g_file, c_file, b_folder)

    def executeTestcase(self, testcase):
        testcase.fail("Executor not defined")
        return False


class BlockTestExecutor(TestExecutor):

    def __init__(self, hiveapi, rules):
        super(BlockTestExecutor, self).__init__(hiveapi, rules)


    def executeTestcase(self,testcase):
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
        }
        params["HIVE_FORK_HOMESTEAD"] = "20000",
        params["HIVE_FORK_TANGERINE"] = "20000",
        params["HIVE_FORK_SPURIOUS"]  = "20000",

        if self.rules >= RULES_HOMESTEAD:
            params["HIVE_FORK_HOMESTEAD"] = "0",
        if self.rules >= RULES_TANGERINE:
            params["HIVE_FORK_TANGERINE"] = "0",
        if self.rules >= RULES_SPURIOUS:
            params["HIVE_FORK_HOMESTEAD"] = "0",

        node = None
        self.hive.log("Starting node")

        try:
            node = self.hive.newNode(params)
        except Exception, e:
            testcase.fail(["Failed to start node (%s)" % str(e)])
            return False


        self.hive.log("Started node %s" % node)

        try:
            testcase.setNodeInstance(node.nodeId)
            (ok, err ) = self.verifyPreconditions(testcase, node)

            if not ok:
                testcase.fail(["Preconditions failed",[err]])
                #print("Precondition fail - genesis used: \n%s\n" % json.dumps(testcase.genesis()))
                return False

            (ok, err) = self.verifyPostconditions(testcase, node)
            self.hive.debugp("verifyPostconditions returned %s" % ok)

            if not ok: 
                testcase.fail(["Postcondition check failed",err])
                return False

            testcase.success()
            return True

        finally:
            self.hive.killNode(node)



    def verifyPreconditions(self, testcase, node):
        """ Verify preconditions 
        @return (bool isOk, list of error messags) 
        """

        first = node.getBlockByNumber(0)
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
        err = None

        def _verifyEqRaw(v,exp):
            if v != exp:
                return "Found `%s`, expected `%s`"  % (v, exp)
            return None

        req_count_start = node.rpcid
        current = ""
        numaccounts = len(testcase.postconditions())
        if numaccounts > 1000:
            self.hive.log("This may take a while, %d accounts to check postconditions for " % numaccounts)

        for address, poststate_account in testcase.postconditions().items():


            if len(errs) > 9:
                errs.append("Postcondition check aborted due to earlier errors")
                return (len(errs) == 0, errs)

            if (node.rpcid - req_count_start) % 1000 == 0:
                self.hive.log("Verifying poststate, have checked %d items ..." % (node.rpcid - req_count_start))
            
            # Parity fails otherwise...
            if address[:2] != "0x":
                address = "0x%s" % address

            try:
                if 'nonce' in poststate_account:
                    current = "noncecheck"
                    _n = node.getNonce(address)
                    exp = hex2big(poststate_account["nonce"])
                    err = _verifyEqRaw(_n, exp)
                    if err is not None:
                        errs.append("Nonce error (`%s`)" % address)
                        errs.append(err)

                if 'code' in poststate_account:
                    current = "codecheck"
                    _c = node.getCode(address)
                    exp = poststate_account["code"]
                    err = _verifyEqRaw(_c, exp)
                    if err is not None:
                        errs.append("Code error (`%s`)" % address)
                        errs.append(err)


                if 'balance' in poststate_account:
                    current = "balancecheck"
                    _b = node.getBalance(address)
                    exp = hex2big(poststate_account["balance"])
                    err = _verifyEqRaw(_b, exp)
                    if err is not None:
                        errs.append("Balance error (`%s`)" % address)
                        errs.append(err)

                if 'storage' in poststate_account and len(poststate_account['storage']) > 0:

                    numkeys = len(poststate_account['storage'])
                    if numkeys > 1000:
                        self.hive.log("This may take a while, checking storage for %d keys" % numkeys)

                    current = "storagecheck"
                    # Must iterate over storage
                    for _hash,exp in poststate_account['storage'].items():
                        current = "Storage (%s)" % _hash
                        value = node.getStorageAt(address, _hash )

                        if int(value,16) != int(exp,16):
                            err  ="Found `%s`, expected `%s`"  % (value, exp)


                        if err is not None:
                            errs.append("Storage error (`%s`) key `%s`" % (address, _hash))
                            errs.append(err)

                        if len(errs) > 9:
                            errs.append("Postcondition check aborted due to earlier errors")
                            return (len(errs) == 0, errs)

                        if (node.rpcid - req_count_start) % 1000 == 0:
                            self.hive.log("Verifying poststate storage, have checked %d items ..." % (node.rpcid - req_count_start))

            except Exception, e:
                errs.append("Postcondition verification failed on %s @ %s after %d checks: %s" %(current,address, count,  str(e)))
                return (False, errs)


        return (len(errs) == 0, errs)

#class GeneralStateTestExecutor(TestExecutor):
#
#    def __init__(self, hiveapi, rules = RULES_FRONTIER):
#        super(GeneralStateTestExecutor, self).__init__(hiveapi)
#        self.ruleset = rules
#
#    def executeTestcase(self,testcase):
#        """
#        Executes a general-state-test testcase. 
#
#
#        """
#        pass
#
