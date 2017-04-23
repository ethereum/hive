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
        self.raw_genesis = None
        self._skipped = True
        self._timeElapsed = None
        self._message = []
        self.nodeInstance = "N/A"
        self.clientType = "N/A"
        self.required_keys = ["pre","blocks","postState","genesisBlockHeader"]

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
        if "network" not in self.data:
            return default

        defined_sets = {
            "Homestead" : Rules.RULES_HOMESTEAD,
            "Frontier"  : Rules.RULES_FRONTIER,
            "EIP150"    : Rules.RULES_TANGERINE,
            "EIP158"    : Rules.RULES_SPURIOUS,
            "TransitionNet" : Rules.RULES_TRANSITIONNET,
            "Metropolis" : Rules.RULES_METROPOLIS,
            }


        if self.data['network'] in defined_sets:
            return defined_sets[self.data['network']]

        return default

    def get(self, key):
        if key in self.data:
            return self.data[key]
        return None

    def genesis(self, key = None):
        """ Returns the 'genesis' block for this testcase,
        including any alloc's (prestate) required """


        # Genesis block
        if self.raw_genesis is None:

            #We must fix some fields in the genesis, geth is picky about that
            fields_to_fix = ['nonce','coinbase','hash','mixHash','parentHash','receiptTrie','stateRoot','transactionsTrie','uncleHash']
        

            raw_genesis = self.data['genesisBlockHeader']

            for key in fields_to_fix:
                v = raw_genesis[key]
                if len(v) > 2 and v[:2] != '0x':
                    raw_genesis[key] = '0x'+raw_genesis[key]

            # And fix the alloc-section
            alloc = {}

            for addr in self.data['pre']:
                v = self.data['pre'][addr]
                if not addr[:2] == '0x':
                    addr = '0x'+addr
                
                if 'storage' in v.keys():
                    storage = {}
                    for slot,data in v['storage'].items():
                        _slot = padHash(slot)
                        _data = padHash(data)
                        storage[_slot] = _data
                    v['storage'] = storage
                alloc[addr] = v


            raw_genesis['alloc'] = alloc
            self.raw_genesis = raw_genesis

        if key is None:
            return self.raw_genesis

        return self.raw_genesis[key]

    def postconditions(self, key = None):
        postconditions = self.data['postState']

        if key is None:
            return self.data['postState']


        if key in self.data['poststate']:
            return self.data['postState'][key]

        return None


    def blocks(self):
        return self.data['blocks']

    def keys(self):
        return None

    def chain(self):
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
