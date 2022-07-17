#!/bin/bash

tiup cdc cli changefeed create --pd=http://{{ .PDServer }}:2379 --changefeed-id="$1" --sink-uri="kafka://{{ .BROKER }}:9092/topic-name?protocol=avro" --schema-registry=http://{{.SchemaRegistry }}:8081 --config $2
