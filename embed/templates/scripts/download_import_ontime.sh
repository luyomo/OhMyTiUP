#!/bin/bash

usage="$(basename "$0") database table_name start_year start_month end_year end_month -- Import the data from ontime between start_year/start_month and end_year/end_month"
if [ $# -ne 6  ]; then
    echo $usage
    exit 1
fi
database=$1
table_name=$2
start_year=$3
start_month=$4
end_year=$5
end_month=$6

mkdir -p /home/admin/tidb-lightning/data
mkdir -p /home/admin/tidb-lightning/sort

cleanup() {
  rv=$?
  exit $rv
}

INDEX=1

for s in `seq $start_year $end_year`
do

  if [ "$s" -lt "${end_year}" ];
  then
    endMonth=12
  else
    endMonth=$end_month
  fi

  if [ "$s" -gt "${start_year}" ];
  then
    startMonth=1
  else
    startMonth=$start_month
  fi

  for m in `seq $startMonth $endMonth`
  do
    FileIdx=$(printf "%05d" $INDEX)

    wget https://transtats.bts.gov/PREZIP/On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip --no-check-certificate
    trap "cleanup" EXIT

    unzip On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
    trap "cleanup" EXIT

    mv "On_Time_Reporting_Carrier_On_Time_Performance_(1987_present)_${s}_${m}.csv" "${1}.${2}.${FileIdx}.csv";
    trap "cleanup" EXIT

    rm -f readme.html
    sed -i -E '1 s/,$//' "${1}.${2}.${FileIdx}.csv"
    trap "cleanup" EXIT

    mv "${1}.${2}.${FileIdx}.csv" "/home/admin/tidb-lightning/data/${1}.${2}.${FileIdx}.csv"
    trap "cleanup" EXIT

    let INDEX=${INDEX}+1
  done
done

rm -f /tmp/tidb_lightning_checkpoint.pb
tidb-lightning -c /opt/tidb/tidb-lightning.toml
trap "cleanup" EXIT

rm -f On_Time_Reporting_Carrier_On_Time_Performance_1987_present_*.zip
rm -f /home/admin/tidb-lightning/data/*.csv
