#!/bin/bash

{{if eq .TiDBPass "" }}
sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host={{ .TiDBHost }} --mysql-user={{ .TiDBUser }} --mysql-db={{ .TiDBDB }} --mysql-port={{ .TiDBPort }} --threads={{ .Threads }} --tables={{ .NumTables }} --table-size={{ .TableSize }} run
{{else}}
sysbench /usr/share/sysbench/oltp_insert.lua --mysql-host={{ .TiDBHost }} --mysql-user={{ .TiDBUser }} --mysql-password={{ .TiDBPass }} --mysql-db={{ .TiDBDB }} --mysql-port={{ .TiDBPort }} --threads={{ .Threads }} --tables={{ .NumTables }} --table-size={{ .TableSize }} run
{{end}}

