#!/bin/bash

userCnt=$1
cnt=$2
file=$3

function rand(){
  min=$1
  max=$(($2-$min+1))
  num=$(cat /dev/urandom | head -n 10 | cksum | awk -F ' ' '{print $1}')
  echo $(($num%$max+$min)) | awk '{printf "%08d\n", $0;}'
}

rm $3

for idx in $(seq 1 $cnt); do
    payer=$(rand 1 $userCnt)
    receiver=$(rand 1 $userCnt)
    amount=$(rand 1000 10000)

    echo "$idx,'user$payer','user$receiver',$amount" >> $3
done

exit 0
