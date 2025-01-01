#!/bin/bash

#
# This is a little test-script, that can be used for a some trial runs of clients.
#
# This script runs all production-ready tests, but does so with very restrictive options,
# and should thus complete in tens of minutes
#

HIVEHOME="./"

# Store results in temp
RESULTS="/tmp/TestResults"

FLAGS="--loglevel 4"
FLAGS="$FLAGS --results-root $RESULTS "
FLAGS="$FLAGS --sim.parallelism 1 --client.checktimelimit=20s"

echo "Running the quick'n'dirty version of the Hive tests, for local development"
echo "To get the hive viewer up, you can do"
echo ""
echo "  cd $HIVEHOME/hiveviewer && ln -s /tmp/TestResults/ Results && python -m SimpleHTTPServer"
echo ""
echo "And then visit http://localhost:8000/ with your browser. "
echo "Log-files and stuff is available in $RESULTS."
echo ""
echo ""


function run {
  echo "$HIVEHOME> $1"
  (cd $HIVEHOME && $1)
}

function testconsensus {
  client=$1
  echo "$(date) Starting hive consensus simulation [$client]"
  run "./hive --sim ethereum/consensus --client $client --sim.loglevel 6 --sim.testlimit 2 $FLAGS"
}
function testgraphql {
  echo "$(date) Starting graphql simulation [$1]"
  run "./hive --sim ethereum/graphql --client $1 $FLAGS"
}

function testsync {
  echo "$(date) Starting hive sync simulation [$1]"
  run "./hive --sim ethereum/sync --client=$1 $FLAGS"
}

function testdevp2p {
  echo "$(date) Starting p2p simulation [$1]"
  run "./hive --sim devp2p --client $1 $FLAGS"
}

mkdir $RESULTS

# Sync are quick tests
#

# These can successfully sync with themselves
#testsync go-ethereum_latest

# These two are failing - even against themselves
testsync besu_latest       # fails
testsync nethermind_latest # fails

#testsync besu_latest,nethermind_latest

#testsync go-ethereum_latest go-ethereum_stable
#testsync go-ethereum_latest nethermind_latest
#testsync go-ethereum_latest besu_latest

# GraphQL implemented only in besu and geth
#

testgraphql go-ethereum_latest
testgraphql besu_latest


# The devp2p tests are pretty quick -- a few minutes
#testdevp2p go-ethereum_latest
#testdevp2p nethermind_latest
#testdevp2p besu_latest


# These take an extremely long time to run
#testconsensus go-ethereum_latest
#testconsensus nethermind_latest
#testconsensus besu_latest



