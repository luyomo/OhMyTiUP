#!/bin/bash

for i in {1..{{ .NumTables }}}
do
    query="alter table sbtest${i} add column tidb_ts timestamp default current_timestamp "
    mysql -h {{ .TiDBHost  }} -P {{ .TiDBPort  }} -u {{ .TiDBUser  }} {{ .TiDBDB  }} -e "${query}"

    query="alter table sbtest${i} modify column tidb_ts timestamp"
    mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "${query}"

    query="alter table sbtest${i} add column mysql_ts timestamp default current_timestamp "
    mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB  }} -e "${query}"
done
