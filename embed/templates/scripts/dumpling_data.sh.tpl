#!/bin/bash

dumpling -u {{.TiDBUser}} {{if .TiDBPassword}}-p{{.TiDBPassword}}{{end}} -P {{.TiDBPort}} -h {{.TiDBHost}} {{if .DBName}}-B {{.DBName}}{{end}} {{if .TableNames}}-T {{.TableNames}}{{end}} --filetype csv -t 8 -o {{if .OutputDir}}{{.OutputDir}}{{else}}/home/admin/dumpling/data{{end}} -r 200000 -F 256MiB




