global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/tidb/tidb-deploy"
  data_dir: "/home/admin/tidb/tidb-data"
server_configs: 
{{- if (and (.Servers.TiCDC) (gt (len .Servers.TiCDC) 0)) }}
  cdc:
    per-table-memory-quota: 20971520
{{- end }}
  tidb:
    performance.txn-total-size-limit: 10737418240
  pd:
    controller.ltb-max-wait-duration: "2h"
{{- if (and (.Servers.Labels) (gt (len .Servers.Labels) 0)) }}
    replication.location-labels: [ {{ range .Servers.Labels -}} "{{. }}" {{- end }} ]
{{ end }}
{{- if (and (.Servers.Pump) (gt (len .Servers.Pump) 0)) }}
    binlog.enable: true
    binlog.ignore-error: false
{{ end  -}}
{{- if (and (.Servers.PD) (gt (len .Servers.PD) 0)) }}
pd_servers:
  {{- range .Servers.PD }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.Servers.TiDB) (gt (len .Servers.TiDB) 0)) }}
tidb_servers:
  {{- range .Servers.TiDB }}
  - host: {{. }}
    {{- if $.AuditLog }}
    config:
      plugin.dir: {{ $.PluginDir }}
      plugin.load: {{ $.AuditLog }}
    {{- end }}
  {{- end }}
{{ end }}
{{- if (and (.Servers.TiFlash) (gt (len .Servers.TiFlash) 0)) }}
tiflash_servers:
  {{- range .Servers.TiFlash }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.Servers.TiKV) (gt (len .Servers.TiKV) 0)) }}
tikv_servers:
  {{- range .Servers.TiKV }}
  - host: {{.IPAddress }}
    {{ if gt (len .Labels) 0 -}}
    config:
      server.labels:
      {{- range $k, $v := .Labels }}
        {{ $k }}: {{ $v }}
      {{- end }}
    {{ end }}
  {{- end }}
{{ end  }}
{{- if (and (.Servers.TiCDC) (gt (len .Servers.TiCDC) 0)) }}
cdc_servers:
  {{- range .Servers.TiCDC }}
  - host: {{. }}
  {{- end }}
{{ end  }}
{{- if (and (.Servers.Pump) (gt (len .Servers.Pump) 0)) }}
pump_servers:
  {{- range .Servers.Pump }}
  - host: {{. }}
  {{- end }}
{{ end }}
{{- if (and (.Servers.Monitor) (gt (len .Servers.Monitor) 0)) }}
monitoring_servers:
  {{- range .Servers.Monitor }}
  - host: {{. }}
  {{- end }}
{{- end }}
{{- if (and (.Servers.Grafana) (gt (len .Servers.Grafana) 0)) }}
grafana_servers:
  {{- range .Servers.Grafana }}
  - host: {{. }}
  {{- end }}
{{- end }}
{{- if (and (.Servers.AlertManager) (gt (len .Servers.AlertManager) 0)) }}
grafana_servers:
alertmanager_servers:
  {{- range .Servers.AlertManager }}
  - host: {{. }}
  {{- end }}
{{ end  }}
