#!/bin/bash

kafka-producer-perf-test \
--topic $1 \
--throughput -1 \
--num-records $2 \
--record-size $3 \
--producer-props acks=all bootstrap.servers={{- range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }}
