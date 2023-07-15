global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/tidb/tidb-deploy"
  data_dir: "/home/admin/tidb/tidb-data"
server_configs: 
{{- if (and (.TiCDC) (gt (len .TiCDC) 0)) }}
  cdc:
    per-table-memory-quota: 20971520
{{- end }}
  tidb:
    performance.txn-total-size-limit: 107374182400
{{- if (and (.Labels) (gt (len .Labels) 0)) }}
  pd:
    replication.location-labels: [ {{ range .Labels -}} "{{. }}" {{- end }} ]
{{ end }}
{{- if (and (.Pump) (gt (len .Pump) 0)) }}
    binlog.enable: true
    binlog.ignore-error: false
{{ end  -}}
{{- if (and (.PD) (gt (len .PD) 0)) }}
pd_servers:
  {{- range .PD }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.TiDB) (gt (len .TiDB) 0)) }}
tidb_servers:
  {{- range .TiDB }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.TiFlash) (gt (len .TiFlash) 0)) }}
tiflash_servers:
  {{- range .TiFlash }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.TiKV) (gt (len .TiKV) 0)) }}
tikv_servers:
  {{- range .TiKV }}
  - host: {{. }}
  {{- end }}
{{ end  }}
{{- if (and (.TiCDC) (gt (len .TiCDC) 0)) }}
cdc_servers:
  {{- range .TiCDC }}
  - host: {{. }}
  {{- end }}
{{ end  }}
{{- if (and (.Pump) (gt (len .Pump) 0)) }}
pump_servers:
  {{- range .Pump }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.Monitor) (gt (len .Monitor) 0)) }}
monitoring_servers:
  {{- range .Monitor }}
  - host: {{. }}
  {{- end }}
{{- end }}
{{- if (and (.Grafana) (gt (len .Grafana) 0)) }}
grafana_servers:
  {{- range .Grafana }}
  - host: {{. }}
  {{- end }}
{{- end }}
{{- if (and (.AlertManager) (gt (len .AlertManager) 0)) }}
grafana_servers:
alertmanager_servers:
  {{- range .AlertManager }}
  - host: {{. }}
  {{- end }}
{{ end  }}
