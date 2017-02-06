import os


def canonicalize(v):
    if type(v) == str or type(v) == unicode:
        v = v.lower()
        if v.startswith("0x"):
            return str(v[2:])
    return v
#files = [ f for f in os.listdir( os.curdir ) if os.path.isfile(f) ]
getFiles = lambda root : [ "%s/%s" % (root,f) for f in os.listdir( root ) if (os.path.isfile("%s/%s" % (root,f)) and f.endswith(".json"))] 

def getFilesRecursive(root):
    for subdir, dirs, files in os.walk(root):
        for fil in files:
            filepath = subdir + os.sep + fil
            if filepath.endswith(".json"):
                yield filepath 





def hex2big(txt):
    txt = canonicalize(txt)

    return int(txt,16)

if __name__ == '__main__':
    ## Testing
    print os.listdir( "./tests/BlockchainTests" )
    for x in getFiles("./tests/BlockchainTests/"):
        print x
    print "--"
    for x in getFilesRecursive("./"):
        print x