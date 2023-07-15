#!/bin/bash

source ~/.bashrc

if [ ${#1} -lt 2 ]
then
  echo "run_mysql_query.sh DBNAME 'select 1'"
  exit 2
fi
dbName=$1
shift
/opt/mysql-shell/bin/mysqlsh {{.DBUser}}:{{.DBPassword}}@{{.DBHost}}:{{.DBPort}}/$dbName --result-format=json --json=pretty --sql --show-warnings=false -e "$1" | jq --slurp '.[1].rows'
# todo
#/opt/mysql-shell/bin/mysqlsh {{.DBUser}}:{{.DBPassword}}@{{.DBHost}}:{{.DBPort}}/$dbName --result-format=json --sql --show-warnings=false -e "$1" 

exit $?
