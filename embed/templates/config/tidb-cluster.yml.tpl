global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/tidb/tidb-deploy"
  data_dir: "/home/admin/tidb/tidb-data"
server_configs: 
{{ if gt (len .TiCDC) 0 }}
  cdc:
    per-table-memory-quota: 20971520
{{ end  }}
  tidb:
    performance.txn-total-size-limit: 107374182400
{{ if gt (len .Pump) 0 }}
    binlog.enable: true
    binlog.ignore-error: false
{{ end  }}
{{ if gt (len .PD) 0 }}
pd_servers:
  {{- range .PD }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{ if gt (len .TiDB) 0 }}
tidb_servers:
  {{- range .TiDB }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{ if gt (len .TiKV) 0 }}
tikv_servers:
  {{- range .TiKV }}
  - host: {{. }}
  {{- end }}
{{ end  }}
{{ if gt (len .TiCDC) 0 }}
cdc_servers:
  {{- range .TiCDC }}
  - host: {{. }}
  {{- end }}
{{ end  }}
{{ if gt (len .Pump) 0 }}
pump_servers:
  {{- range .Pump }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{ if gt (len .Monitor) 0 }}
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
{{ end  }}
