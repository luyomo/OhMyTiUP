source-id: "{{ .SourceName }}"
enable-gtid: false

from:
  host: "{{ .MySQLHost }}"
  user: "{{ .MySQLUser }}"
  password: "{{ .MySQLPassword }}"
  port: {{ .MySQLPort }}
