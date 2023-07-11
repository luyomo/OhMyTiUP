#!/bin/bash

dev=nvme0n1
lsblk /dev/$dev

if [ "$?" -eq "0" ];
then
        fstype=$(lsblk -fs /dev/${dev}p1 -o FSTYPE -n | xargs)
        if [ "$?" -eq 0 ] && [ "$fstype" != "" ] ;
        then
            exit 0
        fi
        printf 'o\nn\np\n1\n\n\nt\n83\nw\n' | fdisk /dev/$dev
        sleep 2
        mkfs.ext4 /dev/${dev}p1
        sleep 2
        mount /dev/${dev}p1 {{.MOUNT_DIR}}
fi

dev=xvdb
lsblk /dev/$dev

if [ "$?" -eq "0" ];
then
        fstype=$(lsblk -fs /dev/${dev}1 -o FSTYPE -n | xargs)
        if [ "$?" -eq 0 ] && [ "$fstype" != "" ] ;
        then
            exit 0
        fi
        printf 'o\nn\np\n1\n\n\nt\n83\nw\n' | fdisk /dev/$dev
        sleep 2
        mkfs.ext4 /dev/${dev}1
        sleep 2
        mount /dev/${dev}1 {{.MOUNT_DIR}}
fi
