#!/bin/bash

if [ $# -eq 0  ]
then
  echo "Please run the command as below"
  echo "kafka-util.sh list-topic"
  echo "kafka-util.sh topic-offset"
  echo "kafka-util.sh sink-status sink-name"
  exit 1
fi

brokerList={{ range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }}
connectorIP={{ range $idx, $data := .Connector -}}{{if eq $idx 0}}{{$ata}}{{end}}{{- end }}

case $1 in
list-topic)
    kafka-topics --list --bootstrap-server=$brokerList;;
topic-offset)
    kafka-run-class kafka.tools.GetOffsetShell --broker-list $brokerList --topic test_test01 --time -1;;
sink-status)
    curl http://$connectorIP:8083/connectors/$2/status | jq
esac
