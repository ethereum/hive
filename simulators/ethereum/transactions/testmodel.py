import json

class Rules():

    RULES_FRONTIER = {
        "HIVE_FORK_HOMESTEAD" : 2000,
        "HIVE_FORK_TANGERINE" : 2000,
        "HIVE_FORK_SPURIOUS"  : 2000,
        "HIVE_FORK_DAO_BLOCK" : 2000,
        "HIVE_FORK_METROPOLIS": 2000, 
    }

    RULES_HOMESTEAD = {

        "HIVE_FORK_HOMESTEAD" : 0,
        "HIVE_FORK_TANGERINE" : 2000,
        "HIVE_FORK_SPURIOUS"  : 2000,
        "HIVE_FORK_DAO_BLOCK" : 2000,
        "HIVE_FORK_METROPOLIS": 2000, 
    }

    RULES_TANGERINE = {
        "HIVE_FORK_HOMESTEAD" : 0,
        "HIVE_FORK_TANGERINE" : 0,
        "HIVE_FORK_SPURIOUS"  : 2000,
        "HIVE_FORK_DAO_BLOCK" : 2000,
        "HIVE_FORK_METROPOLIS": 2000, 
    }
    RULES_SPURIOUS = {

        "HIVE_FORK_HOMESTEAD" : 0,
        "HIVE_FORK_TANGERINE" : 0,
        "HIVE_FORK_SPURIOUS"  : 0,
        "HIVE_FORK_DAO_BLOCK" : 2000,
        "HIVE_FORK_METROPOLIS": 2000, 
    }

    RULES_TRANSITIONNET = {
        "HIVE_FORK_HOMESTEAD" : 5,
        "HIVE_FORK_DAO_BLOCK" : 8,
        "HIVE_FORK_TANGERINE" : 10,
        "HIVE_FORK_SPURIOUS"  : 14,
        "HIVE_FORK_METROPOLIS": 2000, 
    }

    RULES_METROPOLIS = {
        "HIVE_FORK_HOMESTEAD" : 0,
        "HIVE_FORK_TANGERINE" : 0,
        "HIVE_FORK_SPURIOUS"  : 0,
        "HIVE_FORK_DAO_BLOCK" : 0,
        "HIVE_FORK_METROPOLIS": 0, 
    }
# Model for the testcases
class Testfile(object):

    def __init__(self,fname):
        self.filename = fname
        self._tests = []

    def tests(self):
        with open(self.filename,"r") as infile:
            json_data = json.load(infile)
            for k,v in json_data.items():
                t = Testcase(k,v,self)
                self._tests.append(t)
                yield t

    def __str__(self):
        return self.filename

def padHash(data):

    if(data[:2] == '0x'):
        data = data[2:]

    return "0x"+data.zfill(64)

class Testcase(object):

    def __init__(self,name, jsondata, testfile):
        self.name = name
        self.testfile = testfile
        self.data = jsondata
#        self.raw_genesis = None
        self._skipped = True
        self._timeElapsed = None
        self._message = []
        self.nodeInstance = "N/A"
        
        self.required_keys = ["rlp"]


        self.raw_genesis =  {
            "bloom" : "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "coinbase" : "0x0000000000000000000000000000000000000001",
            "difficulty" : "0x0386a0",
            "extraData" : "0x42",
            "gasLimit" : "0x2fefd8",
            "gasUsed" : "0x00",
#            "hash" : "0x91883942c38db663f9dc11baa88af36ebb8512e04ea69f5f275b294902c52604",
#            "mixHash" : "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "nonce" : "0x0102030405060708",
            "number" : "0x00",
            "parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
            "receiptTrie" : "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
#            "stateRoot" : "0x2b9f478fe39744a8c17eb48ae7bf86f6a47031a823a632f9bd661b59978aeefd",
#            "timestamp" : "0x54c98c81",
            "transactionsTrie" : "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "uncleHash" : "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
            "alloc" : {}
        }

    def genesis(self):
        return self.raw_genesis

    def __str__(self):
        return self.name

    def fullname(self):
        return "%s:%s" % (self.testfile, self.name)
        
    def setNodeInstance(self, instanceId):
        self.nodeInstance = instanceId

    def validate(self):
        """Validates that the provided json contains the necessary data
        to perform the test

        @return (ok , msg)
        """
        missing_keys = []
        for k in self.required_keys:
            if k not in self.data.keys():
                missing_keys.append(k)


        return (len(missing_keys) == 0 ,"Missing keys: %s" % (",".join(missing_keys)))

    def ruleset(self, default=Rules.RULES_FRONTIER):
        """In some cases (newer tests), the ruleset is specified in the
        testcase json
        If so, it's returned. Otherwise, default is returned
        """

        if 'blocknumber' in self.data:
            blnum = int(self.data['blocknumber'])

            if blnum >= 3000000: 
                return Rules.RULES_METROPOLIS
            elif blnum >= 2675000:
                return Rules.RULES_TANGERINE
            elif blnum >= 1000000: 
                return Rules.RULES_HOMESTEAD
            else:
                return Rules.RULES_FRONTIER

            return Rules.RULES_FRONTIER


    def get(self, key):
        if key in self.data:
            return self.data[key]
        return None


    def addMessage(self, msg):
        if msg is None:
            return

        if type(msg) != list:
            msg = [msg]

        if len(msg) == 0:
            return

        self._message.extend(msg)

    def fail(self, message):
        """Set if this test failed"""
        self._success = False
        self.addMessage(message)

        self._skipped = False
        print("%s failed : %s " %(self, str(self._message)))

    def success(self, message = []):
        self._success = True
        self._skipped = False
        self.addMessage(message)

    def skipped(self, message = []):
        self._skipped = True
        self.addMessage(message)

    def shouldSucceed(self):
        return self.get('sender') is not None

    def wasSuccessfull(self):
        return bool(self._success)

    def wasSkipped(self):
        return self._skipped

    def topLevelError(self):
        if self._message is not None and len(self._message) > 0:
            return self._message[0]

        return None

    def details(self):
        _d = { "instanceid" : self.nodeInstance}
        if self._message is not None:
            _d["errors"] = self._message
        if self._timeElapsed is not None:
            _d['ms'] = self._timeElapsed
        return _d

    def setTimeElapsed(self,timeInMillis):
        self._timeElapsed = timeInMillis

    def status(self):

        if self._skipped:
            return "skipped"
        if self._success:
            return "success"

        return "failed"
