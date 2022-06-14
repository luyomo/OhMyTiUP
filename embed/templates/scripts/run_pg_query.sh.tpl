#!/bin/bash

if [ ${#1} -lt 2 ]
then
  echo "run_pg_query DBNAME 'select 1'"
  exit 2
fi
dbName=$1
shift
PGPASSWORD={{.DBPassword}} psql -q -t -h {{.DBHost}} -U {{.DBUser}} -p{{.DBPort}} -d $dbName -c "$@"
