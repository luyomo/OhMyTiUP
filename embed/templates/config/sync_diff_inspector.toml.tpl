check-thread-count = 4
export-fix-sql = true
check-struct-only = false
dm-addr = "http://{{ .DMMasterAddr }}"
dm-task = "{{ .TaskName }}"

[task]
    output-dir = "/tmp/output/config"
    target-check-tables = [{{- range $i, $v := .Databases -}} {{if $i}},{{end}} "{{$v}}.*" {{- end -}}]
