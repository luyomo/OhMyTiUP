#!/bin/bash

kafka-run-class kafka.tools.EndToEndLatency {{range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end -}} ssl-perf-test $1 $2 $3
