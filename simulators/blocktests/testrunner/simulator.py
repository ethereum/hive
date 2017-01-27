#!/usr/bin/env python
import os,sys
from hivemodel import HiveNode, HiveAPI

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
    if 'HIVE_SIMULATOR' not in os.environ:
        print("Running in TEST-mode")
        return test()
    hivesim = os.environ['HIVE_SIMULATOR']
    print("Hive simulator: %s\n" % hivesim)
    hive = HiveAPI(hivesim)

    hive.blockTests(start = 7, end=200, blacklist = ["newChainFrom6Block"])

if __name__ == '__main__':
    main(sys.argv[1:])
    
