#!/bin/bash
set -e

if [ $# -ne 3 ]; then
    echo "Prepares custom Harmony built with your local EthereumJ"
    echo "Usage: "
    echo "$0 [EthereumJ directory] [Ethereum Harmony directory, downloaded if absent] [Output directory]"
    echo ""
    echo "Example: $0 ~/ethereumj ~/ethereum-harmony ~"
    echo "You will get harmony.ether.camp.tar (tared Ethereum Harmony built with local EthereumJ) in output directory when script finished"
    exit 1
fi

ETHEREUMJ=$1
ETHEREUM_HARMONY=$2
OUTPUT_DIR=$3

(cd $ETHEREUMJ && ./gradlew install -x test)
if [ -d $ETHEREUM_HARMONY ] ; then
  (cd $ETHEREUM_HARMONY && git checkout develop && git pull)
else
  (cd $(dirname $ETHEREUM_HARMONY) && git clone --depth 1 -b develop https://github.com/ether-camp/ethereum-harmony.git)
fi
(cd $ETHEREUM_HARMONY && ./gradlew dependencyManagement stage -x test -PuseMavenLocal)
cp $ETHEREUM_HARMONY/build/distributions/harmony.ether.camp.tar $OUTPUT_DIR/harmony.ether.camp.tar
echo "$OUTPUT_DIR/harmony.ether.camp.tar written"