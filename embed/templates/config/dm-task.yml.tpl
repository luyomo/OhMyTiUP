name: "{{ .TaskName }}"
task-mode: incremental
meta-schema: "dm_meta"

target-database:
  host: "{{ .Host }}"
  port: {{ .Port }}
  user: "{{ .User }}"
  password: "{{ .Password }}"
  max-allowed-packet: 67108864

routes:
 {{- range $i, $v := .Databases }}
  route-rule-{{$i}}:
    schema-pattern: "{{$v}}"
    target-schema: "{{$v}}"
 {{- end }}

mysql-instances:
  -
    source-id: "{{ .SourceID }}"
    meta:
      binlog-name: {{ .BinlogName }}
      binlog-pos: {{ .BinlogPos }}
    route-rules: [ {{- range $i, $v := .Databases -}} {{if $i}},{{end}} "route-rule-{{$i}}" {{- end -}} ]

