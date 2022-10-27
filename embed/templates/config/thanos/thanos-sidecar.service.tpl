[Unit]
Description=Thanos SideCar
After=network-online.target

[Service]
Restart=on-failure
ExecStart=/opt/thanos-0.28.0.linux-amd64/thanos sidecar \
    --grpc-address=0.0.0.0:29191 \
    --tsdb.path /home/admin/tidb/tidb-data/prometheus-9090 \
    --prometheus.url "http://localhost:9090" \
    --objstore.config-file=/opt/thanos/etc/s3.config.yaml

[Install]
WantedBy=multi-user.target
