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

for s in `seq $start_year $end_year`
do
for m in `seq $start_month $end_month`
do
wget https://transtats.bts.gov/PREZIP/On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip --no-check-certificate
unzip On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
mv "On_Time_Reporting_Carrier_On_Time_Performance_(1987_present)_${s}_${m}.csv" "${1}.${2}.csv";
rm -f readme.html
sed -i -E '1 s/,$//' "${1}.${2}.csv"
mv "${1}.${2}.csv" /home/admin/tidb-lightning/data/

tidb-lightning -c /opt/tidb/tidb-lightning.toml
rm On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
rm "/home/admin/tidb-lightning/data/${1}.${2}.csv"
done
done
