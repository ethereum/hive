from multiprocessing.dummy import Pool as ThreadPool 
import requests
import traceback,sys,os
import binascii
import json
from testmodel import Testcase, Testfile, Rules
from utils import canonicalize, getFiles, hex2big
import time


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

    def getClientversion(self):
        return self._getNodeData("web3_clientVersion",[])

    def getBlockByNumber(self,blnum):
        return self._getNodeData("eth_getBlockByNumber", [str(int(blnum)),True])

    def getLatestBlock(self):
        return self._getNodeData("eth_getBlockByNumber", ["latest",True])


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
        self.count = 0

    def inc(self):
        self.count = self.count+1

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


    def blockTests(self, start =0 , end = -1, whitelist = [], blacklist = [], testfiles = [],executor= None) :
        return self._performTests(start,end,whitelist, blacklist, testfiles, executor)

    def mkTestcaseIterator(self, start = 0, end = -1, whitelist = [], blacklist =[], testfiles=[],executor= None):
        
        hive = self

        def iterator():
            count = 0
            for testfile in testfiles:

                count = count +1
                if count < start:
                    continue
                if 0 <= end <= count:
                    break

                tf = Testfile(testfile)
                hive.log("Commencing testfile [%d] (%s)\n " % (count, tf))

                for testcase in tf.tests() :

                    if len(whitelist) > 0 and str(testcase) not in whitelist:
                        testcase.skipped(["Testcase not in whitelist"])
                        continue

                    if len(blacklist) > 0 and str(testcase) in blacklist:
                        testcase.skipped(["Testcase in blacklist"])
                        continue

                    (ok, err) = testcase.validate()

                    if not ok:
                        hive.log("%s failed initial validation" % testcase )
                        testcase.fail(["Testcase failed initial validation", err])
                    else:
                        yield testcase

            
        return iterator


    def _performTests(self, start = 0, end = -1, whitelist = [], blacklist =[], testfiles=[],executor= None):

        iterator = self.mkTestcaseIterator(start, end, whitelist, blacklist,testfiles, executor)
        hive = self

        def perform_work(testcase):


            start = time.time()
            executor.executeTestcase(testcase)
            end = time.time()
            testcase.setTimeElapsed(1000 * (end - start))
            hive.log("Test: %s %s (%s)" % (testcase.testfile, testcase, testcase.status()))
            hive.subresult(
                    testcase.fullname(),
                    testcase.wasSuccessfull(),
                    testcase.topLevelError(),
                    testcase.details()
                )


        pool = ThreadPool(7) 
        #Turns out a raw iterator isn't supported, so comprehending a list instead :(
        pool.map(perform_work, [x for x in iterator()])
        pool.close()
        pool.join()
            
        return True

    def newNode(self, params):
        try:
            _id = self._post("/nodes", params)
            _ip = self._get("/nodes/%s" % _id)
            return HiveNode(_id, _ip)
        except Exception, e:
            self.log("Failed to start node, trying again")

        _id = self._post("/nodes", params)
        _ip = self._get("/nodes/%s" % _id)
        return HiveNode(_id, _ip)


    def killNode(self,node):
        self._delete("/nodes/%s" % node.nodeId)

class TestExecutor(object):
    """ Test-execution engine

    This should probably be moved into 'testmodel' instead. 

    """
    def __init__(self, hiveapi, rules = None):
        self.hive = hiveapi
        self.default_rules = rules

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

    def __init__(self, hiveapi, rules = None):
        super(BlockTestExecutor, self).__init__(hiveapi, rules)
        self.clientVersion = None

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
            "HIVE_FORK_DAO_VOTE" : "1",
        }
        params["HIVE_FORK_HOMESTEAD"] = "20000",
        params["HIVE_FORK_TANGERINE"] = "20000",
        params["HIVE_FORK_SPURIOUS"]  = "20000",


        if self.default_rules is not None:
            params.update(self.default_rules)
        else:
            params.update(testcase.ruleset())

        node = None
        self.hive.log("Starting node for test %s" % testcase)
        
        try:
            node = self.hive.newNode(params)
        except Exception, e:
            testcase.fail(["Failed to start node (%s)" % str(e)])
            return False

        if self.clientVersion is None:
            self.clientVersion = node.getClientversion()
            print("Client version: %s" % self.clientVersion)

        #self.hive.log("Started node %s" % node)

        try:
            testcase.setNodeInstance(node.nodeId)
            (ok, err ) = self.verifyPreconditions(testcase, node)

            if not ok:
                testcase.fail(["Preconditions failed",[err]])
                #testcase.addMessage(self.customCheck(testcase, node))
                return False

            (ok, err) = self.verifyPostconditions(testcase, node)
            #self.hive.debugp("verifyPostconditions returned %s" % ok)

            if not ok: 
                testcase.fail(["Postcondition check failed",err])
                return False

            testcase.success()
            return True

        finally:
            self.hive.killNode(node)

    def customCheck(self, testcase, node):
        """This is a special method meant for debugging particular testcases in hive, 
        not meant to be run in general"""

        value = node.getStorageAt("0xaaaf5374fce5edbc8e2a8697c15331677e6ebf0b", "0x010340fef9c35e91836ea450d2e0b39079f7ac19da70f533a0c9a6770d6d8efc" )
        value2 = node.getStorageAt("0xaaaf5374fce5edbc8e2a8697c15331677e6ebf0b", "0x00" )
        errs = [
            "Checked storage 0xaaaf5374fce5edbc8e2a8697c15331677e6ebf0b / 0x010340fef9c35e91836ea450d2e0b39079f7ac19da70f533a0c9a6770d6d8efc",
            "Got %s, expected %s" % (value , "0x0516afa543fbe239a5a78a4588f77f82aee7f22d"),
            "Checked storage 0xaaaf5374fce5edbc8e2a8697c15331677e6ebf0b / 0x00",
            "Got %s, expected %s" % (value2 , "0x1f40") ]

        return errs
        
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

        # With a bit of luck, we can just check the `lastblockhash` directly
        exp_lastblockhash = testcase.get("lastblockhash")
        if exp_lastblockhash is not None: 
            actual_lastblockhash = node.getLatestBlock()['hash']
            err = _verifyEqRaw(actual_lastblockhash[-64:],exp_lastblockhash[-64:])
            #TODO, run with whitelist ['DaoTransactions_EmptyTransactionAndForkBlocksAhead']
            # on parity, to see why that test is passing!
            if err is not None:
                errs.append("Last block hash wrong")
                errs.append([err])
            else:
                return (len(errs) == 0, errs)

        # Either 'lastblockhash' is missing, or it isn't right. Continue checking to debug what's wrong

        req_count_start = node.rpcid
        current = ""
        numaccounts = len(testcase.postconditions())
        if numaccounts > 1000:
            self.hive.log("This may take a while, %d accounts to check postconditions for " % numaccounts)

        for address, poststate_account in testcase.postconditions().items():


            if len(errs) > 9:
                errs.append("Postcondition check aborted due to earlier errors")
                return (len(errs) == 0, errs)

            if (node.rpcid - req_count_start) % 1000 == 0 and (node.rpcid - req_count_start) > 0:
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
                        errs.append([err])

                if 'code' in poststate_account:
                    current = "codecheck"
                    _c = node.getCode(address)
                    exp = poststate_account["code"]
                    err = _verifyEqRaw(_c, exp)
                    if err is not None:
                        errs.append("Code error (`%s`)" % address)
                        errs.append([err])


                if 'balance' in poststate_account:
                    current = "balancecheck"
                    _b = node.getBalance(address)
                    exp = hex2big(poststate_account["balance"])
                    err = _verifyEqRaw(_b, exp)
                    if err is not None:
                        errs.append("Balance error (`%s`)" % address)
                        errs.append([err])

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
                            errs.append([err])

                        if len(errs) > 9:
                            errs.append("Postcondition check aborted due to earlier errors")
                            return (len(errs) == 0, errs)

                        if (node.rpcid - req_count_start) % 1000 == 0:
                            self.hive.log("Verifying poststate storage, have checked %d items ..." % (node.rpcid - req_count_start))

            except Exception, e:
                errs.append("Postcondition verification failed on %s @ %s after %d checks: %s" %(current,address, count,  str(e)))
                return (False, errs)


        return (len(errs) == 0, errs)
