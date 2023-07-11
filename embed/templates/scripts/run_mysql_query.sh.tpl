#!/bin/bash

if [ $# -lt 2 ]
then
  echo "run_mysql_query.sh DBNAME 'select 1'"
  echo "run_mysql_query.sh DBNAME DBUser DBPassword 'select 1'"
  exit 2
fi

if [ $# -eq 4 ]
then
  dbName=$1
  shift
  dbUser=$1
  shift
  dbPassword=$1
  shift
elif [ $# -eq 2 ]
then
  dbName=$1
  shift
fi

if [ -z "$dbUser" ];
then
  mysql -s -N -h {{.DBHost}} -P {{.DBPort}} -u {{.DBUser}} {{if .DBPassword}}-p{{.DBPassword}}{{end}} $dbName <<EOF
$@
EOF
else
  mysql -s -N -h {{.DBHost}} -P {{.DBPort}} -u $dbUser -p$dbPassword $dbName <<EOF
$@
EOF
fi

exit $?
