#!/bin/sh

set -xe

geas -bin -no-push0 contracts/deployer.eas > bytecode/deployer.bin
geas -bin -no-push0 contracts/callenv.eas > bytecode/callenv.bin
geas -bin -no-push0 contracts/callme.eas > bytecode/callme.bin
geas -bin -no-push0 contracts/callrevert.eas > bytecode/callrevert.bin
geas -bin -no-push0 contracts/emit.eas > bytecode/emit.bin
geas -bin -no-push0 contracts/genlogs.eas > bytecode/genlogs.bin
geas -bin -no-push0 contracts/gencode.eas > bytecode/gencode.bin
geas -bin -no-push0 contracts/genstorage.eas > bytecode/genstorage.bin
