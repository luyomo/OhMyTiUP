[Unit]
Description=Thanos Store
After=network-online.target

[Service]
Restart=on-failure
ExecStart=/opt/thanos-0.28.0.linux-amd64/thanos query \
   {{- range .StoreServers }}
    --store        {{. }}:29191 \
    --store        {{. }}:39191 \
   {{- end }}
    --http-address 0.0.0.0:49090 \
    --grpc-address 0.0.0.0:49191 

[Install]
WantedBy=multi-user.target
