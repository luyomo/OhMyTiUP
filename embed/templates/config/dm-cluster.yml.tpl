global:
  user: "admin"
  ssh_port: 22
  deploy_dir: "/home/admin/dm/dm-deploy"
  data_dir: "/home/admin/dm/dm-data"

monitored:
  node_exporter_port: 19100
  blackbox_exporter_port: 19115
  deploy_dir: /home/admin/dm/dm-deploy/monitor-19100
  data_dir: /home/admin/dm/dm-data/monitor-19100
  log_dir: /home/admin/dm/dm-deploy/monitor-19100/log

server_configs:
  master:
    log-level: info
  worker:
    log-level: info

master_servers:
   {{- range $idx, $node := .DMMaster }}
  - host: {{ $node }}
    name: master{{$idx}}
    ssh_port: 22
    port: 8261
    config:
      log-level: info
   {{- end }}

worker_servers:
   {{- range .DMWorker }}
  - host: {{. }}
    ssh_port: 22
    port: 8262
    config:
      log-level: info
   {{- end }}
{{- if (and (.Monitor) (gt (len .Monitor) 0)) }}
monitoring_servers:
   {{- range .Monitor }}
  - host: {{. }}
    port: 19090
   {{- end }}
{{- end }}
{{- if (and (.Monitor) (gt (len .Monitor) 0)) }}
grafana_servers:
   {{- range .Monitor }}
  - host: {{. }}
    port: 13000
   {{- end }}
{{- end }}
{{- if (and (.Monitor) (gt (len .Monitor) 0)) }}
alertmanager_servers:
   {{- range .Monitor }}
  - host: {{. }}
    web_port: 19093
    cluster_port: 19094
   {{- end }}
{{- end }}
