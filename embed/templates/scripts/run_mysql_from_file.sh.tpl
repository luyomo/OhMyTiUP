#!/bin/bash

if [ ${#1} -lt 2 ]
then
  echo "run_mysql_query.sh DBNAME file-name"
  exit 2
fi

mysql -h {{.DBHost}} -P {{.DBPort}} -u {{.DBUser}} -p{{.DBPassword}} $1 < $2
