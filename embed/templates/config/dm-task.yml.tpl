name: "{{ .TaskMetaData.TaskName }}"
task-mode: incremental
meta-schema: "dm_meta"

target-database:
  host: "{{ .TaskMetaData.Host }}"
  port: {{ .TaskMetaData.Port }}
  user: "{{ .TaskMetaData.User }}"
  password: "{{ .TaskMetaData.Password }}"
  max-allowed-packet: 67108864

routes:
 {{- range $i, $v := .TaskMetaData.Databases }}
  route-rule-{{$i}}:
    schema-pattern: "{{$v}}"
    target-schema: "{{$v}}"
 {{- end }}

mysql-instances:
  -
    source-id: "{{ .TaskMetaData.SourceID }}"
    meta:
      binlog-name: {{ .TaskMetaData.BinlogName }}
      binlog-pos: {{ .TaskMetaData.BinlogPos }}
    route-rules: [ {{- range $i, $v := .TaskMetaData.Databases -}} {{if $i}},{{end}} "route-rule-{{$i}}" {{- end -}} ]

