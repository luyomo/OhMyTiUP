#!/bin/bash

wget https://download.oracle.com/otn_software/linux/instantclient/214000/instantclient-basic-linux.x64-21.4.0.0.0dbru.zip
wget https://download.oracle.com/otn_software/linux/instantclient/214000/instantclient-sqlplus-linux.x64-21.4.0.0.0dbru.zip
unzip instantclient-basic-linux.x64-21.4.0.0.0dbru.zip
unzip instantclient-sqlplus-linux.x64-21.4.0.0.0dbru.zip

for host in {{ .OracleClientServers }} 
do
  ssh $host 'sudo mkdir -p /opt/oracle'
  scp -r instantclient_21_4 $host:/tmp/
  ssh $host 'sudo rm -rf /opt/oracle/instantclient_21_4'
  ssh $host 'sudo mv /tmp/instantclient_21_4 /opt/oracle'
  ssh $host "grep 'instantclient_21_4' ~/.bashrc || echo 'export LD_LIBRARY_PATH=/opt/oracle/instantclient_21_4:$LD_LIBRARY_PATH' >> ~/.bashrc"
  ssh $host "grep 'export PATH=\$LD_LIBRARY_PATH:\$PATH' ~/.bashrc || echo 'export PATH=\$LD_LIBRARY_PATH:\$PATH' >> ~/.bashrc"
done
