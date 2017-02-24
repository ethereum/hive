#!/usr/bin/env python
import os,sys
import hivemodel
from testmodel import Rules
import utils

class HiveTestNode(hivemodel.HiveNode):

    def __init__(self, nodeId = None, nodeIp = None):
        self.nodeId ="Testnode"

    def getBlockByNumber(self,blnum):
        return {u'hash':"0x0000", u'stateRoot':"0x0102030405060708"}

    def getNonce(self,address):
        return 10000

    def getBalance(self,address):
        return 1000


    def getCode(self,address):
        return "0xDEADBEEF"


    def getStorageAt(self,address, _hash):
        return "0xDEADBEEF"

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
    executor = hivemodel.BlockTestExecutor(hive , Rules.RULES_TANGERINE)
    hive.blockTests(testfiles= utils.getFiles("./tests/BlockchainTests"), executor = executor)

def main(args):

    print("Simulator started\n")

    if 'HIVE_SIMULATOR' not in os.environ:
        print("Running in TEST-mode")
        return test()

    hivesim = os.environ['HIVE_SIMULATOR']
    print("Hive simulator: %s\n" % hivesim)
    hive = hivemodel.HiveAPI(hivesim)

    status = hive.blockTests(testfiles = utils.getFiles("./tests/BlockchainTests"), 
        executor = hivemodel.BlockTestExecutor(hive , Rules.RULES_FRONTIER))
#        start=0, end=2)

    status = hive.blockTests(testfiles = utils.getFiles("./tests/BlockchainTests/EIP150"),
        executor = hivemodel.BlockTestExecutor(hive , Rules.RULES_TANGERINE))

    status = hive.blockTests(testfiles = utils.getFiles("./tests/BlockchainTests/Homestead"),
            executor = hivemodel.BlockTestExecutor(hive , Rules.RULES_HOMESTEAD))

    status = hive.blockTests(testfiles = utils.getFiles("./tests/BlockchainTests/TestNetwork"),
            executor = hivemodel.BlockTestExecutor(hive , Rules.RULES_TRANSITIONNET))
    
    status = hive.blockTests(testfiles = utils.getFilesRecursive("./tests/BlockchainTests/GeneralStateTests/"), 
        executor = hivemodel.BlockTestExecutor(hive))
#        whitelist=["mload32bitBound_return2_d0g0v0_EIP150"])

    if not status:
        sys.exit(-1)

    sys.exit(0)

if __name__ == '__main__':
    main(sys.argv[1:])
    
