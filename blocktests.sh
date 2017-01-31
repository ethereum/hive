#!/bin/bash

hive -sim blocktest -test none -client *:master  > data.json
curl -o tmp.zip -L https://github.com/holiman/testreport_template/archive/master.zip && unzip tmp.zip && rm tmp.zip

NOW=$(date +"%Y-%d-%m_%H-%M-%S")
REPORTDIR="report-$NOW"

mv ./testreport_template-master/ $REPORTDIR
cp -r ./workspace/logs/simulations/ $REPORTDIR 




#Create jsonp aswell
echo "onData(" > $REPORTDIR/data.jsonp
cat data.json >> $REPORTDIR/data.jsonp
echo ");" >> $REPORTDIR/data.jsonp

cp  data.json $REPORTDIR/

tar -cvzf $REPORTDIR.tar.gz $REPORTDIR/ && rm -rf $REPORTDIR
 
echo "Report generated into $NOW.tar.gz"
