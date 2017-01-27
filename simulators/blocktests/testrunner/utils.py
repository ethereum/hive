import os


def canonicalize(v):
    if type(v) == str or type(v) == unicode:
        v = v.lower()
        if v.startswith("0x"):
            return str(v[2:])
    return v

def getFiles(root, limit = 10):
    #print("Root %s" % root)
    counter = 0
    for subdir, dirs, files in os.walk(root):
        #print("subdir %s" % subdir)
        for fil in files:
            filepath = subdir + os.sep + fil
            if filepath.endswith(".json"):
                yield filepath 
                counter = counter +1
            if counter == limit:
                return

def hex2big(txt):
    txt = canonicalize(txt)

    return int(txt,16)
