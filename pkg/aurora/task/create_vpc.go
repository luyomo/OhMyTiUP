// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"context"
	"fmt"
    //"strings"

    //"github.com/luyomo/tisample/pkg/aurora/ctxt"
    //"github.com/pingcap/errors"
    "github.com/luyomo/tisample/pkg/aurora/executor"
)

// Mkdir is used to create directory on the target host
type CreateVpc struct {
	user string
	host string
}

// Execute implements the Task interface
func (c *CreateVpc) Execute(ctx context.Context) error {
    local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

    stdout, stderr, err := local.Execute(ctx, "aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisampletest\"", false)
    if err != nil {
        fmt.Printf("The error here is <%#v> \n\n", err)
        fmt.Printf("----------\n\n")
        fmt.Printf("The error here is <%s> \n\n", string(stderr))
        return nil
    }
    fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
    return nil

    stdout, stderr, err = local.Execute(ctx, "aws ec2 create-vpc --cidr-block 172.80.0.0/16 --tag-specifications \"ResourceType=vpc,Tags=[{Key=Name,Value=tisampletest}]\"", false)
    if err != nil {
        fmt.Printf("The error here is <%#v> \n\n", err)
        fmt.Printf("----------\n\n")
        fmt.Printf("The error here is <%s> \n\n", string(stderr))
        return nil
    }
    fmt.Printf("The output from ls is <%s> \n\n\r\r", stdout)
    stdout, stderr, err = local.Execute(ctx, "aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=tisampletest\"", false)
    if err != nil {
        fmt.Printf("The error here is <%#v> \n\n", err)
        fmt.Printf("----------\n\n")
        fmt.Printf("The error here is <%s> \n\n", string(stderr))
        return nil
    }
    return nil
}

// Rollback implements the Task interface
func (c *CreateVpc) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateVpc) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}


//{
//    "Vpc": {
//        "CidrBlock": "172.80.0.0/16",
//        "DhcpOptionsId": "dopt-d74aa6b0",
//        "State": "pending",
//        "VpcId": "vpc-01de9b7fca24bbd6e",
//        "OwnerId": "385595570414",
//        "InstanceTenancy": "default",
//        "Ipv6CidrBlockAssociationSet": [],
//        "CidrBlockAssociationSet": [
//            {
//                "AssociationId": "vpc-cidr-assoc-0bd2e868b73e690c4",
//                "CidrBlock": "172.80.0.0/16",
//                "CidrBlockState": {
//                    "State": "associated"
//                }
//            }
//        ],
//        "IsDefault": false,
//        "Tags": [
//            {
//                "Key": "Name",
//                "Value": "tisampletest"
//            }
//        ]
//    }
//}
//


//{
//    "Vpcs": [
//        {
//            "CidrBlock": "172.80.0.0/16",
//            "DhcpOptionsId": "dopt-d74aa6b0",
//            "State": "available",
//            "VpcId": "vpc-01de9b7fca24bbd6e",
//            "OwnerId": "385595570414",
//            "InstanceTenancy": "default",
//            "CidrBlockAssociationSet": [
//                {
//                    "AssociationId": "vpc-cidr-assoc-0bd2e868b73e690c4",
//                    "CidrBlock": "172.80.0.0/16",
//                    "CidrBlockState": {
//                        "State": "associated"
//                    }
//                }
//            ],
//            "IsDefault": false,
//            "Tags": [
//                {
//                    "Key": "Name",
//                    "Value": "tisampletest"
//                }
//            ]
//        }
//    ]
//}

