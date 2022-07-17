#!/bin/bash

kafka-topics --delete -topic $1 --bootstrap-server {{ range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }}

kafka-topics \
--create \
--topic $1 \
--partitions $2 \
--replication-factor 2 \
--config retention.ms=86400000 \
--config min.insync.replicas=2 \
--bootstrap-server {{ range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }} 
