#!/bin/bash

kafka-topics --delete -topic ssl-perf-test --bootstrap-server {{ range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }}

kafka-topics \
--create \
--topic ssl-perf-test \
--partitions $1 \
--replication-factor 3 \
--config retention.ms=86400000 \
--config min.insync.replicas=2 \
--bootstrap-server {{ range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }} 
