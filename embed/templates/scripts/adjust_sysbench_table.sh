#!/bin/bash

for i in {1..50}
do
    query="alter table sbtest${i} add column tidb_ts timestamp default current_timestamp "
    mysql -h 172.83.1.49 -P 4000 -u root cdc_test -e "${query}"

    query="alter table sbtest${i} modify column tidb_ts timestamp"
    mysql -h testtisample.ckcbeq0sbqxz.ap-northeast-1.rds.amazonaws.com -P 3306 -u master -p1234Abcd cdc_test -e "${query}"

    query="alter table sbtest${i} add column mysql_ts timestamp default current_timestamp "
    mysql -h testtisample.ckcbeq0sbqxz.ap-northeast-1.rds.amazonaws.com -P 3306 -u master -p1234Abcd cdc_test -e "${query}"
done
