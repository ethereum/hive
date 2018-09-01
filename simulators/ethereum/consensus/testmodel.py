import json

class Rules():

    RULESETS = {
        "Frontier"  : {
            "HIVE_FORK_HOMESTEAD" : 2000,
            "HIVE_FORK_TANGERINE" : 2000,
            "HIVE_FORK_SPURIOUS"  : 2000,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 2000, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "Homestead" : {
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 2000,
            "HIVE_FORK_SPURIOUS"  : 2000,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 2000, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "EIP150"    : {
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 0,
            "HIVE_FORK_SPURIOUS"  : 2000,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 2000, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "EIP158"    : {
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 0,
            "HIVE_FORK_SPURIOUS"  : 0,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 2000, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "Byzantium" : {
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 0,
            "HIVE_FORK_SPURIOUS"  : 0,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 0, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "Constantinople" : {
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 0,
            "HIVE_FORK_SPURIOUS"  : 0,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 0, 
            "HIVE_FORK_CONSTANTINOPLE" : 0, 
        },
        "FrontierToHomesteadAt5" : {
            "HIVE_FORK_HOMESTEAD" : 5,
            "HIVE_FORK_TANGERINE" : 2000,
            "HIVE_FORK_SPURIOUS"  : 2000,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 2000,             
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "HomesteadToEIP150At5" : {
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 5,
            "HIVE_FORK_SPURIOUS"  : 2000,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 2000, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "HomesteadToDaoAt5":{
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 2000,
            "HIVE_FORK_SPURIOUS"  : 2000,
            "HIVE_FORK_DAO_BLOCK" : 5,
            "HIVE_FORK_BYZANTIUM" : 2000, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "EIP158ToByzantiumAt5":{
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 0,
            "HIVE_FORK_SPURIOUS"  : 0,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 5, 
            "HIVE_FORK_CONSTANTINOPLE" : 2000,
        },
        "ByzantiumToConstantinopleAt5":{
            "HIVE_FORK_HOMESTEAD" : 0,
            "HIVE_FORK_TANGERINE" : 0,
            "HIVE_FORK_SPURIOUS"  : 0,
            "HIVE_FORK_DAO_BLOCK" : 2000,
            "HIVE_FORK_BYZANTIUM" : 0, 
            "HIVE_FORK_CONSTANTINOPLE" : 5,         
      }
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
        self.required_keys = ["pre","blocks","postState","genesisBlockHeader","network"]

    def __str__(self):
        return self.name

    def fullname(self):
        return "%s:%s" % (self.testfile, self.name)
        
    def setNodeInstance(self, instanceId):
        self.nodeInstance = instanceId

    def validateNetwork(self):
        """Returns error message if this test case is not properly 
        configured for a network in the defined ruleset. 
        @return error message or None
        """

        if "network" not in self.data:
            return "Testcase does not have a 'network' specification"
        
        if self.data['network'] not in Rules.RULESETS:     
            return "Network %s not defined in hive ruleset" % self.data['network']

        return None

    def validate(self):
        """Validates that the provided json contains the necessary data
        to perform the test

        @return 'ErrorMessage' if error,
                 None if all correct
        """
        missing_keys = []
        for k in self.required_keys:
            if k not in self.data.keys():
                missing_keys.append(k)


        if len(missing_keys) > 0:
            return "Missing keys: %s" % (",".join(missing_keys))

        #And finally, fail if no ruleset is defined
        return self.validateNetwork()

    def skipPow(self):
        """ Returns True if this testcase should be executed without PoW verification"""
        return "sealEngine" in self.data and self.data['sealEngine'] == 'NoProof'

    def ruleset(self):
        """The ruleset for tests should be specified in the json
        """
        if "network" in self.data and self.data['network'] in Rules.RULESETS:
            return Rules.RULESETS[self.data['network']]

        return None

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
                if addr[:2] == '0x':
                    addr = addr[2:]
                
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
        print("%s %s failed : %s " %(self.testfile, self, str(self._message)))

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
