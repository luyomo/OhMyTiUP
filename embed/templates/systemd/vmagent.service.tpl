[Unit]
Description=High-performance, cost-effective and scalable time series database, long-term remote storage for Prometheus
After=network.target

[Service]
Type=simple
User=root
Group=root
StartLimitBurst=5
StartLimitInterval=0
Restart=on-failure
RestartSec=1
ExecStart=/usr/local/bin/vmagent-prod \
        -promscrape.config=/etc/prometheus.yml \
        -remoteWrite.url=http://{{ .VMEndpoint }}:8480/insert/0/prometheus/api/v1/write
ExecStop=/bin/kill -s SIGTERM $MAINPID
LimitNOFILE=65536
LimitNPROC=32000

[Install]
WantedBy=multi-user.target
