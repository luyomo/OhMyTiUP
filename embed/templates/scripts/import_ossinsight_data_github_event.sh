#!/bin/bash

if [ $# -ne 2  ]; then
    echo "Please fill in two parameters"
    exit 1
fi
StartNum=$1
EndNum=$2

for i in $(seq -f "%05g" $StartNum $EndNum)
do
  wget -P /tmp/ https://ossinsight-data.s3.amazonaws.com/parquet/github_events.part_$i
  /opt/scripts/run_mysql_query ossinsight "LOAD DATA LOCAL INFILE '/tmp/github_events.part_$i' INTO TABLE github_events"
  rm /tmp/github_events.part_$i
done

