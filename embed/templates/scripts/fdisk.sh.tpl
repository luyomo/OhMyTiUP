#!/bin/bash

dev=nvme0n1
lsblk /dev/$dev

if [ "$?" -eq "0" ];
then
        fstype=$(lsblk -fs /dev/${dev}p1 -o FSTYPE -n)
        if [ "$?" -eq 0 ];
        then
            exit 0
        fi
        printf 'o\nn\np\n1\n\n\nt\n83\nw\n' | fdisk /dev/$dev
        mkfs.ext4 /dev/${dev}p1
        mount /dev/${dev}p1 /home/admin/tidb
fi

