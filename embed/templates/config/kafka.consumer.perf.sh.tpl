#!/bin/bash

kafka-consumer-perf-test \
--topic $1 \
--broker-list {{range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }} \
--messages $2 | jq -R .|jq -sr 'map(./",")|transpose|map(join(": "))[]'
