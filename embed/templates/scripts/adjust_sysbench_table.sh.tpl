#!/bin/bash

x=1
while [ $x -le 200 ]
do
    query="select count(*) from information_schema.tables where table_schema = '{{ .MySQLDB  }}' and table_name like 'sbtest%'"
    result=`mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB }} -s --skip-column-name -e "${query}"`
    if [ $result -eq {{ .NumTables }} ] ;
    then
        break
    fi
    sleep 5
    x=$(( $x + 1 ))
done

for i in {1..{{ .NumTables }}}
do
    query="alter table sbtest${i} add column mysql_ts timestamp default current_timestamp "
    mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "${query}"

    query="alter table sbtest${i} add column tidb_ts timestamp default current_timestamp "
    mysql -h {{ .TiDBHost  }} -P {{ .TiDBPort  }} -u {{ .TiDBUser  }} {{ .TiDBDB  }} -e "${query}"

done
