from multiprocessing.dummy import Pool as ThreadPool
import requests
import traceback,os
import binascii
import json
from testmodel import Testcase, Testfile, Rules
from utils import canonicalize, getFiles, hex2big
import time

# Number of tests to run in parallel.
PARALLEL_TESTS = 16

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

    def subresult(self,nodeid, name, success, errormsg, details = None ):
        params = {
                "name" : name,
                "nodeid" : nodeid,
                "success" : success
        }
        if errormsg is not None:
            params["error"] = errormsg

        data = None
        if details is not None:
            data = {"details" : json.dumps(details) }

        return self._post("/subresults", params = params, data = data);
    
    def clients(self):
        _jsonclients= self._get("/clients")
        _clients  = json.loads(_jsonclients)
     
        return _clients

    def newNode(self, params):
        try:
            _id = self._post("/nodes", params)
            _ip = self._get("/nodes/%s" % _id)
            return HiveNode(_id, _ip)
        except Exception, e:
            self.log("Failed to start node[1], trying again")

        try:
            _id = self._post("/nodes", params)
            _ip = self._get("/nodes/%s" % _id)
            return HiveNode(_id, _ip)
        except Exception, e:
            self.log("Failed to start node[2], trying again (last attempt)")

        _id = self._post("/nodes", params)
        _ip = self._get("/nodes/%s" % _id)
        return HiveNode(_id, _ip)

    def killNode(self,node):
        self._delete("/nodes/%s" % node.nodeId)


class BlockTestExecutor(object):

    def __init__(self, hive_api, testfiles):
        self.clientVersion = None
        self.hive = hive_api
        self.testfiles = testfiles
        self.clients=self.hive.clients()

    def run(self, start=0 , end=-1, whitelist=[], blacklist=[]) :
        return self._performTests(start, end, whitelist, blacklist)

    def makeTestcasesWrapped(self, start=0, end=-1, whitelist=[], blacklist=[]):
        try:
            return self.makeTestcases(start, end, whitelist, blacklist)
        except Exception as e:
            print("Exception while making testcases!")
            print(str(e))
            print(traceback.format_exc())

    def makeTestcases(self, start=0, end=-1, whitelist=[], blacklist=[]):
        count = 0
        for testfile in self.testfiles:
            count = count +1
            if count < start:
                continue
            if 0 <= end <= count:
                break

            tf = Testfile(testfile)
#            self.hive.log("Commencing testfile [%d] (%s)\n " % (count, tf))
            
            #apply the testcases to each available client type
            for client in self.clients:
                for testcase in tf.tests() :
                    
                    if len(whitelist) > 0 and str(testcase) not in whitelist:
                        testcase.skipped(["Testcase not in whitelist"])
                        continue

                    if len(blacklist) > 0 and str(testcase) in blacklist:
                        testcase.skipped(["Testcase in blacklist"])
                        continue

                    err = testcase.validateNetwork()
                    if err != None:
                        testcase.skipped([err])
                        continue

                    err = testcase.validate()

                    testcase.clientType= client

                    if err is not None:
                        self.hive.log("%s / %s failed initial validation" % (tf, testcase) )
                        testcase.fail(["Testcase failed initial validation", err])
                    else:
                        yield testcase

    def _startNodeAndRunTestWrapped(self, testcase):
        try:
            return self._startNodeAndRunTest(testcase)
        except Exception as e:
            print("Exception while starting/running node")
            print(str(e))
            print(traceback.format_exc())


    def reportTest(self, tc, start, nodeId = "NA"):

        tc.setTimeElapsed(1000 * ( time.time() - start))
        self.hive.log("Test: %s %s (%s)" % (tc.testfile, tc, tc.status()))
        self.hive.subresult(nodeId,
                tc.fullname(),
                tc.wasSuccessfull(),
                tc.topLevelError(),
                tc.details()
            )
    def _startNodeAndRunTest(self, testcase):
        start = time.time()
        try:
            genesis, init_chain, blocks = self._generateArtefacts(testcase)
        except:
            testcase.fail(["Failed to write test data to disk", traceback.format_exc()])
            return
        #HIVE_INIT_GENESIS path to the genesis file to seed the client with (default = "/genesis.json")
        #HIVE_INIT_CHAIN path to an initial blockchain to seed the client with (default = "/chain.rlp")
        #HIVE_INIT_BLOCKS path to a folder of blocks to import after seeding (default = "/blocks/")
        #HIVE_INIT_KEYS path to a folder of account keys to import after init (default = "/keys/")
        #HIVE_FORK_HOMESTEAD

        clientName= testcase.clientType

        params = {
            "CLIENT":clientName,
            "HIVE_INIT_GENESIS": genesis,
            "HIVE_INIT_BLOCKS" : blocks,
            "HIVE_FORK_DAO_VOTE" : "1",
        }

        params.update(testcase.ruleset())

        if testcase.skipPow():
            params["HIVE_SKIP_POW"] = "1"

        self.hive.log("Starting client node for test %s" % testcase)

        try:
            node = self.hive.newNode(params)
        except Exception, startNodeError:
            try:
                error = traceback.format_exc()
            except Exception, e:
                error = str(startNodeError)
            testcase.fail(["Failed to start client node", traceback.format_exc()])
            self.reportTest(testcase, start)
            return

        self.executeTestcase(testcase, node)
        self.reportTest(testcase, start, nodeId = node.nodeId)

        try:
            self.hive.killNode(node)
        except Exception, killNodeError:
            # Clients are sometimes already killed by timeouts
            pass
            #try:
            #    error = traceback.format_exc()
            #except Exception, e:
            #    error = str(killNodeError)

            #self.hive.log("Failed to kill node %s: %s" % (node, error))

    def _performTests(self, start=0, end=-1, whitelist=[], blacklist=[]):
        pool = ThreadPool(PARALLEL_TESTS)
        testgenerator = self.makeTestcasesWrapped(start, end, whitelist, blacklist)
        pool.map(lambda test: self._startNodeAndRunTestWrapped(test),testgenerator)
        pool.close()
        pool.join()
        # FIXME: Return false if any tests fail.

    def _generateArtefacts(self, testcase):
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
            try:
                for block in testcase.blocks():
                    b_file = "./artefacts/%s/blocks/%04d.rlp" % (testcase, counter)
                    binary_string = binascii.unhexlify(block['rlp'][2:])
                    with open(b_file,"wb+") as outf:
                        outf.write(binary_string)
                    counter = counter +1
            except TypeError, e:
                #Bad rlp
                self.hive.log("Exception: %s, continuing regardless" % e)

        return (g_file, c_file, b_folder)

    def executeTestcase(self, testcase, node):
        print("Checking version")
        try:
            if self.clientVersion is None:
                self.clientVersion = node.getClientversion()
        except Exception, e:
            print("Failed to get client version")
            print(str(e))

        testcase.setNodeInstance(node.nodeId)
        errors = self.verifyPreconditions(testcase, node)
        if errors:
            testcase.fail(["Preconditions failed", errors])
            return

        errors = self.verifyPostconditions(testcase, node)
        if errors:
            testcase.fail(["Postcondition check failed", errors])
            return

        testcase.success()

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
        @return list of error messages
        """
        errs = []
        try:
            first = node.getBlockByNumber(0)
        except Exception, e:
            errs.append("Failed to get first block")
            errs.append(str(e))
            return errs

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

        return errs

    def verifyPostconditions(self, testcase, node):
        """ Verify postconditions
        @return list of error messages
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
            if err is not None:
                errs.append("Last block hash wrong")
                errs.append([err])

            # To make the hive-runs go faster, we can just stop here. 
            # Either the last blockhash is right (everything ok), or the last blockhash 
            # is wrong (fail.)

            return errs

        # Either 'lastblockhash' is missing, or it isn't right. Continue checking to debug what's wrong

        req_count_start = node.rpcid
        current = ""
        numaccounts = len(testcase.postconditions())
        if numaccounts > 1000:
            self.hive.log("This may take a while, %d accounts to check postconditions for " % numaccounts)

        for address, poststate_account in testcase.postconditions().items():

            if len(errs) > 9:
                errs.append("Postcondition check aborted due to earlier errors")
                return errs

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
                            return errs

                        if (node.rpcid - req_count_start) % 1000 == 0:
                            self.hive.log("Verifying poststate storage, have checked %d items ..." % (node.rpcid - req_count_start))

            except Exception, e:
                errs.append("Postcondition verification failed on %s @ %s: %s" %(current,address, str(e)))
                return errs

        return errs
