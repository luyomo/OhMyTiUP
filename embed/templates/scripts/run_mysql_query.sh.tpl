#!/bin/bash

if [ ${#1} -lt 2 ]
then
  echo "run_mysql_query.sh DBNAME 'select 1'"
  exit 2
fi
dbName=$1
shift
mysql -s -N -h {{.DBHost}} -P {{.DBPort}} -u {{.DBUser}} {{if .DBPassword}}-p{{.DBPassword}}{{end}} $dbName <<EOF
$@
EOF

exit $?
