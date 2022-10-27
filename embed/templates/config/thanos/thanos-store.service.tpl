[Unit]
Description=Thanos Store
After=network-online.target

[Service]
Restart=on-failure
ExecStart=/opt/thanos-0.28.0.linux-amd64/thanos store \
    --data-dir             /var/thanos/store \
    --objstore.config-file /opt/thanos/etc/s3.config.yaml \
    --http-address         0.0.0.0:39090 \
    --grpc-address         0.0.0.0:39191

[Install]
WantedBy=multi-user.target
