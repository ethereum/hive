#!/bin/bash

hive -sim blocktest -test none -client .*:master  -loglevel 6 > data.json
curl -o tmp.zip -L https://github.com/holiman/testreport_template/archive/master.zip && unzip tmp.zip && rm tmp.zip

REPORTDIR="testreport"
NOW=$(date +"%Y-%d-%m_%H-%M-%S")

mv ./testreport_template-master/ $REPORTDIR
cp -r ./workspace/logs/simulations/ $REPORTDIR 




#Create jsonp aswell
echo "onData(" > data.jsonp
cat data.json >> data.jsonp
echo ");" >> data.jsonp

mv  data.jsonp $REPORTDIR/
mv  data.json $REPORTDIR/

tar -cvzf $NOW.tar.gz $REPORTDIR/ # && rm -rf $REPORTDIR
 
echo "Report generated into $NOW.tar.gz"
