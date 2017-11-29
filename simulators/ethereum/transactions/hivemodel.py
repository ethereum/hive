from multiprocessing.dummy import Pool as ThreadPool
import requests
import traceback,os
import binascii
import json
from testmodel import Testcase, Testfile, Rules
from utils import canonicalize, getFiles, hex2big
import time

# Number of tests to run in parallel.
PARALLEL_TESTS = 1




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
        
        response = self.session.post(self.url,json = payload).json()
        
        if 'result' in response:
            return response['result']
        else:
            raise Exception("Error getting node data; payload=%s, response=%s" % (
                payload, response['error']))

    def getClientversion(self):
        return self._getNodeData("web3_clientVersion",[])

    def getBlockByNumber(self,blnum):
        return self._getNodeData("eth_getBlockByNumber", [hex(int(blnum)),True])

    def getTransactionByHash(self,h):
        return self._getNodeData("eth_getTransactionByHash", [h])

    def getLatestBlock(self):
        return self._getNodeData("eth_getBlockByNumber", ["latest",True])

    def sendRawTransaction(self, hexdata):
        return self._getNodeData("eth_sendRawTransaction",[hexdata])

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
        self.activeNode = None
        self.activeNodeKey = ""
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


    def transactionTests(self, start=0 , end=-1, whitelist=[], blacklist=[], testfiles=[], executor=None) :
        return self._performTests(start, end, whitelist, blacklist, testfiles, executor)

    def makeTestcases(self, start=0, end=-1, whitelist=[], blacklist=[], testfiles=[], executor=None):
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

                if not ok:
                    self.log("%s failed initial validation" % testcase )
                    testcase.fail(["Testcase failed initial validation", err])
                else:
                    yield testcase



    def _getNodeForTest(self, testcase, executor):

        executor.hive = self

        start = time.time()
        nodekey = str(testcase.ruleset())
        if nodekey == self.activeNodeKey:
            return self.activeNode

        if self.activeNode is not None:
            self.killNode(self.activeNode)

        try:
            genesis, init_chain, blocks = self._generateArtefacts(testcase)
        except Exception, e:
            testcase.fail(["Failed to write test data to disk", traceback.format_exc()])
            return

        params = {
            "HIVE_INIT_GENESIS": genesis,
            "HIVE_INIT_BLOCKS" : blocks,
            "HIVE_FORK_DAO_VOTE" : "1",
            "HIVE_FORK_HOMESTEAD" : "2000",
            "HIVE_FORK_TANGERINE" : "2000",
            "HIVE_FORK_SPURIOUS" : "2000"
        }
        params.update(testcase.ruleset())

        self.log("Starting client node for test %s" % testcase)
        
        try:
            node = self.newNode(params)
            self.activeNodeKey = nodekey
            self.activeNode = node
            return node
        except Exception, e:
            testcase.fail(["Failed to start client node", traceback.format_exc()])

        return None

    def _startNodeAndRunTest(self, testcase, executor):
        node = self._getNodeForTest(testcase, executor)

        start = time.time()        
        if node is not None: 
            executor.executeTestcase(testcase, node)
        end = time.time()

        testcase.setTimeElapsed(1000 * (end - start))
        self.log("Test: %s %s (%s)" % (testcase.testfile, testcase, testcase.status()))
        self.subresult(
                testcase.fullname(),
                testcase.wasSuccessfull(),
                testcase.topLevelError(),
                testcase.details()
            )
#        try:
#            self.killNode(node)
#        except Exception, e:
#            self.log("Failed to kill node %s: %s" % (node, traceback.format_exc()))

    def _performTests(self, start=0, end=-1, whitelist=[], blacklist=[], testfiles=[], executor=None):
        pool = ThreadPool(PARALLEL_TESTS)
        pool.map(lambda test: self._startNodeAndRunTest(test, executor),
                 self.makeTestcases(start, end, whitelist, blacklist, testfiles, executor))
        pool.close()
        pool.join()
        # FIXME: Return false if any tests fail.

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

    def _generateArtefacts(self, testcase):

        g_file = "./artefacts/%s/genesis.json" % testcase        
        if testcase.genesis() is not None:
            try:
               os.makedirs("./artefacts/%s/" % testcase)
            except Exception, e:
                pass

            with open(g_file,"w+") as g:
                json.dump(testcase.genesis(),g)

        return (g_file, None, None)


class TransactionTestExecutor(object):
 
    def executeTestcase(self, testcase, node):
        
        testcase.setNodeInstance(node.nodeId)

        errors = self.verifyPreconditions(testcase, node)
        if errors:
            testcase.fail(["Preconditions failed", errors])
            return

        try:
            node.sendRawTransaction(testcase.get('rlp'))
        except Exception, e:
            if testcase.shouldSucceed():
                testcase.fail(["Execution failed",str(e),["Testcase sender", testcase.get("sender")]])
            else:
                testcase.success()

            return
            
        errors = self.verifyPostconditions(testcase, node)
        if errors:
            testcase.fail(["Postcondition check failed", errors])
            return

        testcase.success()

    def verifyPreconditions(self, testcase, node):
        """ Verify preconditions
        @return list of error messages
        """
        return []

    def verifyPostconditions(self, testcase, node):
        """ Verify postconditions
        @return list of error messages
        """
        errs = []


        h = testcase.get('hash')

        result = node.getTransactionByHash("0x%s" % h)
        errs.append[result]
        return errs

