#!/bin/bash

if [ ${#1} -lt 2 ]
then
  echo "run_mysql_query.sh DBNAME 'select 1'"
  exit 2
fi
dbName=$1
shift
start=`date +%s`
mysql -s -N -h {{.DBHost}} -P {{.DBPort}} -u {{.DBUser}} -p{{.DBPassword}} $dbName -e "$@" 
end=`date +%s`
expr "nd - $start"
