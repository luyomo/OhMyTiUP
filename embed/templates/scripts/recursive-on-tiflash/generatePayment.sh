#!/bin/bash

userCnt=$1
cnt=$2
function rand(){
  min=$1
  max=$(($2-$min+1))
  num=$(cat /dev/urandom | head -n 10 | cksum | awk -F ' ' '{print $1}')
  echo $(($num%$max+$min)) | awk '{printf "%08d\n", $0;}'
}

#/opt/scripts/run_tidb_query test "drop table payment"
#/opt/scripts/run_tidb_query test "create table payment(id bigint primary key auto_random, payer varchar(32), receiver varchar(32), pay_amount bigint)"

for idx in $(seq 1 $cnt); do
    payer=$(rand 1 $userCnt)
    receiver=$(rand 1 $userCnt)
    amount=$(rand 1000 10000)

    /opt/scripts/run_tidb_query test "insert into payment(payer, receiver, pay_amount) values('user$payer', 'user$receiver', $amount )"
done

exit 0
