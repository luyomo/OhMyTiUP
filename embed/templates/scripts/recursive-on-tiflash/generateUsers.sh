#!/bin/bash

cnt=$1

#/opt/scripts/run_tidb_query test "drop table users "
#/opt/scripts/run_tidb_query test "create table users (id bigint primary key auto_increment, name varchar(32))"

for idx in $(seq -f "%08g" 1 $cnt); do
    /opt/scripts/run_tidb_query test "insert into users(id, name) values($idx, 'user$idx')"
done
