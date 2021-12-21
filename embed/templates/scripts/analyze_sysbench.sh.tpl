#!/bin/bash

mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "create table if not exists cdc_latency(table_name varchar(32) primary key, latency decimal(20,4) )"
mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "delete from cdc_latency"

mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "create table if not exists cdc_qps_data(table_name varchar(128) not null primary key, min_ts timestamp, max_ts timestamp, count bigint)"
mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "delete from cdc_qps_data"

tidbCnt=0
for i in {1..{{ .NumTables }}}
do
    query="select count(*) from sbtest${i}"
    {{ if eq .TiDBPass "" }}
    result=`mysql -h {{ .TiDBHost }} -P {{ .TiDBPort  }} -u {{ .TiDBUser  }} {{ .TiDBDB }} -s --skip-column-names -e "${query}"`
    {{ else  }}
    result=`mysql -h {{ .TiDBHost }} -P {{ .TiDBPort  }} -u {{ .TiDBUser  }} -p{{ .TiDBPass }}  {{ .TiDBDB }} -s --skip-column-names -e "${query}"`
    {{ end  }}
    tidbCnt=$(( $tidbCnt + $result ))
done

x=1
while [ $x -le 200 ]
do
    mysqlCnt=0
    for i in {1..{{ .NumTables }}}
    do
        query="select count(*) from sbtest${i}"
        result=`mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB }} -s --skip-column-names -e "${query}"`
        mysqlCnt=$(( $mysqlCnt + $result ))
    done

    if [ $tidbCnt -eq $mysqlCnt ] ;
    then
        break
    fi
    sleep 5
    x=$(( $x + 1 ))
done

for i in {1..{{ .NumTables }}}
do
    query="insert into cdc_latency select 'sbtest${i}',  sum(time_to_sec(timediff(mysql_ts, tidb_ts)))/count(*) as latency from sbtest${i} "
    mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "${query}"

    query="insert into cdc_qps_data select 'sbtest${i}', min(tidb_ts), max(mysql_ts), count(*) from sbtest${i}"
    mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "${query}"
done


query="select time_to_sec( timediff(max(max_ts), min(min_ts))  ) as time, sum(count)/time_to_sec( timediff(max(max_ts), min(min_ts))  ) as qps,  sum(count) as count from cdc_qps_data"
mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -s --skip-column-names -e "${query}"

query="select sum(latency)/count(*) as latency from cdc_latency"
mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -s --skip-column-names -e "${query}"
