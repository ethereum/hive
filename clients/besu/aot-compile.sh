#!/bin/bash

# This script builds a shared library for most of besu using the JEP 295 ahead-of-time
# compiler.
#
# The output files can be used to speed up starting besu by
# passing them to the java runtime like this:
#
#  -XX:AOTLibrary=/opt/besu/besuAOT.so -XX:AOTLibrary=/opt/besu/javaBaseAOT.so

set -e

cd /opt/besu/lib
classpath=$(ls -1 $PWD/*.jar | paste -sd ':' -)

aotjars="$(echo besu-*.jar picocli-*.jar tuweni-*.jar jackson-*.jar log4j-*.jar guava*.jar bcprov-*.jar rocksdb-*)"
jaotc --ignore-errors --output /opt/besu/besuAOT.so -J-cp -J$classpath --jar $aotjars
jaotc --ignore-errors --output /opt/besu/javaBaseAOT.so --module java.base
