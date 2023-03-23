[Unit]
Description=cdc service
After=syslog.target network.target remote-fs.target nss-lookup.target

[Service]
Environment="AWS_DEFAULT_REGION={{ .AWS_REGION }}"
Environment="AWS_ACCESS_KEY_ID={{ .AWS_ACCESS_KEY_ID }}"
Environment="AWS_SECRET_ACCESS_KEY={{ .AWS_SECRET_ACCESS_KEY }}"
LimitNOFILE=1000000
LimitSTACK=10485760
User=admin
ExecStart=/bin/bash -c '/home/admin/tidb/tidb-deploy/cdc-8300/scripts/run_cdc.sh'
Restart=always

RestartSec=15s

[Install]
WantedBy=multi-user.target
