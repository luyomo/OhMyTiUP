#!/bin/bash

database=$1
sleeptime=$2
num_loop=$3

/opt/scripts/run_tidb_query $database "delete from test01 "
for i in $(seq 1 $num_loop)
do
    sleep $sleeptime
    /opt/scripts/run_tidb_query $database "insert into test01 values($i,1,'This is the test message ')" 2>/dev/null
done
