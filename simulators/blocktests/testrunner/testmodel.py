import json

# Model for the testcases
class Testfile(object):

    def __init__(self,fname):
        self.filename = fname
        self._tests = []

    def tests(self):
        with open(self.filename,"r") as infile: 
            json_data = json.load(infile)
            for k,v in json_data.items():
                t = Testcase(k,v)
                self._tests.append(t)
                yield t

    def getReport(self, clienttype):
        outp = []

        skipped = []
        failed = []
        success = []
        for test in self._tests:
            if test.wasSkipped(): 
                skipped.append(test)
            elif not test.wasSuccessfull():
                failed.append(test)
            else:
                success.append(test)

        outp.append("\n# %s %s\n" % (clienttype, self) )
        outp.append("Success: %d / Fail: %d / Skipped: %d\n" % (len(success), len(failed), len(skipped)))

        def x(l,title):
            if len(l) > 0:
                outp.append("\n## %s\n" % title)
                for test in l:
                    outp.append("* %s" % test.getReport())


        x(failed , "Failed")
        x(skipped, "Skipped")
        x(success, "Successfull")

        return "\n".join(outp)
        
    def __str__(self):
        return "File `%s`" % self.filename



class Testcase(object):

    def __init__(self,name, jsondata):
        self.name = name
        self.data = jsondata
        self.raw_genesis = None
        self._skipped = True
        self._message = []
        self.nodeInfo = "N/A"

    def __str__(self):
        return self.name

    def setNodeInfo(self, nodeInfo):
        self.nodeInfo = nodeInfo

    def validate(self):
        required_keys = ["pre","blocks","postState","genesisBlockHeader"]
        missing_keys = []
        for k in required_keys:
            if k not in self.data.keys():
                missing_keys.append(k)

        
        return (len(missing_keys) == 0 ,"Missing keys: %s" % (",".join(missing_keys))) 

    def genesis(self, key = None):
        # Genesis block
        if self.raw_genesis is None:
            raw_genesis = self.data['genesisBlockHeader']

            # Turns out the testcases have noncewritten as 0102030405060708. 
            # Which is supposed to be interpreted as 0x0102030405060708. 
            # But if it's written as 0102030405060708 in the genesis file, 
            # it's interpreted differently. So we'll need to mod that on the fly 
            # for every testcase.
            nonce = raw_genesis[u'nonce']
            if not raw_genesis[u'nonce'][:2] == '0x':
                raw_genesis[u'nonce'] = '0x'+raw_genesis[u'nonce']

            raw_genesis['alloc'] = self.data['pre']
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

    def fail(self, message):
        """Set if this test failed"""
        self._success = False
        self._message = message
        self._skipped = False
    
    def success(self, message = []):
        self._success = True
        self._message = message
        self._skipped = False

    def skipped(self, message = []):
        self._skipped = True
        self._message = message


    def wasSuccessfull(self):
        return bool(self._success)

    def wasSkipped(self):
        return self._skipped


    def report(self):
        if self.wasSuccessfull():
            print("%s: Success" % self.name)
            return

        if self.wasSkipped():
            print("%s: Skipped")
        else:
            print("%s: Failed" % self.name)
        
        for msg in self._message:
            print("  %s" % msg)

    def getReport(self):
        outp = ["%s (%s)" % (self.name, self.status())]

        if self._message is not None:
            for msg in self._message:
                if type(msg) == list:
                    for _m in msg: 
                        outp.append("    * %s" % str(_m))
                else:
                    outp.append("   * %s" % str(msg))
  
            outp.append("  * Executed on %s" % self.nodeInfo)

        return "\n".join(outp)

    def status(self):

        if self._skipped:
            return "skipped"
        if self._success:
            return "success"

        return "failed"
