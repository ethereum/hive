#!/usr/bin/env python
import os,sys
import hivemodel
from testmodel import Rules
import utils

def main(args):

    print("Simulator started\n")

    if 'HIVE_SIMULATOR' not in os.environ:
        print("Running in TEST-mode")
        return test()

    hivesim = os.environ['HIVE_SIMULATOR']
    print("Hive simulator: %s\n" % hivesim)
    hive = hivemodel.HiveAPI(hivesim)

    status = hive.transactionTests(testfiles=utils.getFilesRecursive("./tests/TransactionTests/"), 
        executor=hivemodel.TransactionTestExecutor(),
        blacklist=["String10MbData","TRANSCT__ZeroByteAtRLP_2"])

    if not status:
        sys.exit(-1)

    sys.exit(0)

if __name__ == '__main__':
    main(sys.argv[1:])
