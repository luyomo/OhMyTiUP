global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/tidb/tidb-deploy"
  data_dir: "/home/admin/tidb/tidb-data"
server_configs: {}
pd_servers:
  {{- range .PD }}
  - host: {{. }}
  {{- end }}
tidb_servers:
  {{- range .TiDB }}
  - host: {{. }}
  {{- end }}
tikv_servers:
  {{- range .TiKV }}
  - host: {{. }}
  {{- end }}
cdc_servers:
  {{- range .TiCDC }}
  - host: {{. }}
  {{- end }}
monitoring_servers:
  {{- range .Monitor }}
  - host: {{. }}
  {{- end }}
grafana_servers:
  {{- range .Monitor }}
  - host: {{. }}
  {{- end }}
alertmanager_servers:
  {{- range .Monitor }}
  - host: {{. }}
  {{- end }}
