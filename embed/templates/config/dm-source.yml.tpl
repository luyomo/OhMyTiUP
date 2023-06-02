source-id: "{{ .SourceName }}"
enable-gtid: false

from:
  host: "{{ .Host }}"
  user: "{{ .User }}"
  password: "{{ .Password }}"
  port: {{ .Port }}
