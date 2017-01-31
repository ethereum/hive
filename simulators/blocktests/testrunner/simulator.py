#!/usr/bin/env python
import os,sys
import hivemodel
import utils

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

class HiveTestNode(hivemodel.HiveNode):

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
        return "Node[test]@test"

class HiveTestAPI(hivemodel.HiveAPI):

    def __init__(self):
        super(hivemodel.HiveAPI, self).__init__()


    def _get(self,path, params = None):
        return "foo"

    def _post(self,path, params = None):
        return "foo"

    def newNode(self, params):
        return HiveTestNode()

    def killNode(self, node):
        pass
    def generateArtefacts(self,testcase):
        return (None, None, None)

    def subresult(self, name, success, errormsg, errors = None ):
        print("subresult: \n\t%s\n\t%s\n\t%s\n\t%s" % (name, success, errormsg, errors))


    def log(self,msg):
        print("LOG: %s" % msg)


def test():
    hive = HiveTestAPI()
    executor = hivemodel.BlockTestExecutor(hive , hivemodel.RULES_TANGERINE)
    hive.blockTests(testfiles= utils.getFiles("./tests/BlockchainTests"), executor = executor)

def main(args):

    print("Validator started\n")

    if 'HIVE_SIMULATOR' not in os.environ:
        print("Running in TEST-mode")
        return test()

    hivesim = os.environ['HIVE_SIMULATOR']
    print("Hive simulator: %s\n" % hivesim)
    hive = hivemodel.HiveAPI(hivesim)

    #executor = hivemodel.BlockTestExecutor(hive , hivemodel.RULES_FRONTIER)

    #hive.blockTests(start = 0, testfiles= utils.getFiles("./tests/BlockchainTests"), executor = executor)
#        whitelist = ["newChainFrom6Block"])

    executor = hivemodel.BlockTestExecutor(hive , hivemodel.RULES_TANGERINE)


    status = hive.blockTests(start = 0, testfiles= utils.getFiles("./tests/BlockchainTests/EIP150"),executor = executor)

    if not status:
        sys.exit(-1)

    sys.exit(0)
    #hive.generalStateTests(start=0, end=2000, whitelist="blockhash0.json", testfiles=utils.getFilesRecursive("./tests/generalStateTests"))

if __name__ == '__main__':
    main(sys.argv[1:])
    
