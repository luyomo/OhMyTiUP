check-thread-count = {{ .CheckThreadCount }}
export-fix-sql = true
check-struct-only = false
dm-addr = "http://{{ .MasterNode }}"
dm-task = "{{ .DMTaskName }}"
[task]
    output-dir = "{{ .DMOutputDir }}"
    target-check-tables = [{{- range $i, $v := .Databases -}} {{if $i}},{{end}} "{{$v}}.*" {{- end -}}]
