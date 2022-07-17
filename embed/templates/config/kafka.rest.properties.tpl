
bootstrap.servers={{- range $idx, $data := .Broker -}}{{if $idx}},{{end}}PLAINTEXT://{{$data}}:9092{{- end }}
