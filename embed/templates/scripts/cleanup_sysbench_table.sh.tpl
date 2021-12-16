#!/bin/bash

mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB }} -s --skip-column-names -e "delete from cdc_latency"

mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB }} -s --skip-column-names -e "delete from cdc_qps_data"

for i in {1..{{ .NumTables }}}
do
  query="truncate table sbtest${i}"
  {{if eq .TiDBPass "" }}
  mysql -h {{ .TiDBHost }} -P {{ .TiDBPort }} -u {{ .TiDBUser }} {{ .TiDBDB }} -e "${query}"
  {{else}}
  mysql -h {{ .TiDBHost }} -P {{ .TiDBPort }} -u {{ .TiDBUser }} -p{{.TiDBPass}} {{ .TiDBDB }} -e "${query}"
  {{end}}
done

x=1
while [ $x -le 40000  ]
do
  cnt=0
  for i in {1..{{ .NumTables }}}
  do
    query="select count(*) from sbtest${i}"
    result=`mysql -h {{ .MySQLHost }} -P {{ .MySQLPort  }} -u {{ .MySQLUser  }} -p{{ .MySQLPass }} {{ .MySQLDB }} -s --skip-column-names -e "${query}"`
    cnt=$(( $cnt + $result ))
  done

  if [ $result -eq 0 ] ;
  then
    break
  fi

  sleep 50
  x=$(( $x + 1 ))
done
