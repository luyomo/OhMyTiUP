#!/bin/bash

kafka-consumer-perf-test \
--topic ssl-perf-test \
--broker-list {{range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }} \
--messages $1 | jq -R .|jq -sr 'map(./",")|transpose|map(join(": "))[]'
