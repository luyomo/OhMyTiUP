#!/bin/bash

kafka-producer-perf-test \
--topic ssl-perf-test \
--throughput -1 \
--num-records $1 \
--record-size $2 \
--producer-props acks=all bootstrap.servers={{- range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }}
