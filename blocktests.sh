#!/bin/bash

hive -sim blocktest -test none -client .*:master  -loglevel 6
curl -L https://github.com/holiman/testreport_template/archive/0.1.tar.gz | tar -xvz

REPORTDIR="`pwd`/testreport"
NOW=$(date +"%Y-%d-%m_%H-%M-%S")

mv ./testreport_template-0.1 $REPORTDIR
cp -r ./workspace/logs $REPORTDIR/
cp  ./workspace/reports/*.json $REPORTDIR/
cp  ./workspace/reports/*.jsonp $REPORTDIR/
tar -cvzf $NOW.tar.gz $REPORTDIR 
echo "Report generated into $REPORTDIR and $NOW.tar.gz.\nCheck out file://$REPORTDIR/index.html"
