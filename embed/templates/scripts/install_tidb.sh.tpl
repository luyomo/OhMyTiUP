#!/bin/bash

rm -f tidb-v6.0.0-linux-amd64.tar.gz
rm -rf tidb-v6.0.0-linux-amd64
wget https://download.pingcap.org/tidb-v6.0.0-linux-amd64.tar.gz
tar xvf tidb-v6.0.0-linux-amd64.tar.gz

for host in {{ .TiDBServers }} 
do
  scp -r tidb-v6.0.0-linux-amd64 $host:/tmp/
  ssh $host 'sudo rm -rf /opt/tidb-v6.0.0-linux-amd64'
  ssh $host 'sudo mv /tmp/tidb-v6.0.0-linux-amd64 /opt/'
  ssh $host "grep 'export PATH=/opt/tidb-v6.0.0-linux-amd64/bin:$PATH' ~/.bashrc || echo 'export PATH=/opt/tidb-v6.0.0-linux-amd64/bin:$PATH' >> ~/.bashrc"
done


