bootstrap.servers=PLAINTEXT://{{- range $idx, $data := .Broker -}}{{if $idx}},{{end}}{{$data}}:9092{{- end }}

group.id=connect-cluster

key.converter=io.confluent.connect.avro.AvroConverter
key.converter.schema.registry.url=http://{{- range $idx, $data := .SchemaRegistry -}}{{if eq $idx 0}}{{$data}}{{end}}{{- end -}}:8081
key.converter.enhanced.avro.schema.support=true
value.converter=io.confluent.connect.avro.AvroConverter
value.converter.schema.registry.url=http://{{- range $idx, $data := .SchemaRegistry -}}{{if eq $idx 0}}{{$data}}{{end}}{{- end -}}:8081
value.converter.enhanced.avro.schema.support=true

key.converter.schemas.enable=true
value.converter.schemas.enable=true

offset.storage.topic=connect-offsets
offset.storage.replication.factor=1

config.storage.topic=connect-configs
config.storage.replication.factor=1

status.storage.topic=connect-status
status.storage.replication.factor=1

rest.advertised.host.name={{ .ConnectorIP  }}

offset.flush.interval.ms=10000

plugin.path=/usr/share/java
