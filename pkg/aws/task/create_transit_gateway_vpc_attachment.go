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
	"encoding/json"
	"errors"
	"fmt"
	//"github.com/luyomo/tisample/pkg/aws/spec"
	//	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//	"go.uber.org/zap"
	//"strings"
	//"time"
)

type TransitGatewayVpcAttachment struct {
	TransitGatewayAttachmentId string `json:"TransitGatewayAttachmentId"`
	VpcId                      string `json:"VpcId"`
	State                      string `json:"State"`
	Tags                       []Tag  `json:"Tags"`
}

type TransitGatewayVpcAttachments struct {
	TransitGatewayVpcAttachments []TransitGatewayVpcAttachment `json:"TransitGatewayVpcAttachments"`
}

/*
"TransitGatewayVpcAttachment": {
        "TransitGatewayAttachmentId": "tgw-attach-0c88bc5f416bbd838",
        "TransitGatewayId": "tgw-06bbd6d3f4b234d74",
        "VpcId": "vpc-00039b67fc75a458c",
        "VpcOwnerId": "385595570414",
        "State": "pending",
        "SubnetIds": [
		"subnet-00bf66cc0f200ee73",
			"subnet-0a262adbaa7d3d1a4",
			"subnet-02e8222b3778f168a"
		],
        "CreationTime": "2021-12-04T13:53:33+00:00",
        "Options": {
		"DnsSupport": "enable",
		"Ipv6Support": "disable",
		"ApplianceModeSupport": "disable"
        },
        "Tags": [
		{
			"Key": "Name",
				"Value": "testtisample"
		},
			{
			"Key": "Cluster",
				"Value": "tisample-tidb2ms"
		},
			{
			"Key": "Type",
				"Value": "tidb"
		}
		]
}
*/

type CreateTransitGatewayVpcAttachment struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

//
// create-transit-gateway --description testtisample --tag-specifications ...
// Execute implements the Task interface
func (c *CreateTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	// 1. transit gateway
	// 2. vpc id
	// 3. subnets
	command := fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\"", c.clusterName, c.clusterType, c.subClusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)

	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}
	var transitGatewayVpcAttachments TransitGatewayVpcAttachments
	fmt.Printf("The result from create-transit-gateway-vpc-attachment <%s> \n\n\n", string(stdout))
	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	fmt.Printf("The vpc attachment data is <%#v> \n\n\n", transitGatewayVpcAttachments.TransitGatewayVpcAttachments)
	if len(transitGatewayVpcAttachments.TransitGatewayVpcAttachments) > 0 {
		return nil
	}

	//

	transitGateway, err := getTransitGateway(local, ctx, c.clusterName)
	if err != nil {
		return err
	}
	if transitGateway == nil {
		return errors.New("No transit gateway found")
	}
	fmt.Printf("The transit gateway is <%#v> \n\n\n", transitGateway)

	vpc, err := getVPC(local, ctx, c.clusterName, c.clusterType, c.subClusterType)
	if err != nil {
		return err
	}
	if vpc == nil {
		return errors.New("No vpc found")
	}
	fmt.Printf("The vpc is <%#v> \n\n\n", vpc)

	subnets, err := getNetworksString(local, ctx, c.clusterName, c.clusterType, c.subClusterType, "private")
	if err != nil {
		return err
	}
	if subnets == "" {
		return errors.New("No subnets found")
	}
	fmt.Printf("The subnets is <%s> \n\n\n", subnets)

	command = fmt.Sprintf("aws ec2 create-transit-gateway-vpc-attachment --transit-gateway-id %s --vpc-id %s --subnet-ids '\"'\"'%s'\"'\"' --tag-specifications \"ResourceType=transit-gateway-attachment,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s}]\"", transitGateway.TransitGatewayId, vpc.VpcId, subnets, c.clusterName, c.clusterType, c.subClusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)

	stdout, stderr, err = local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return err
	}
	var transitGatewayVpcAttachment TransitGatewayVpcAttachment
	fmt.Printf("The result from create-transit-gateway-vpc-attachment <%s> \n\n\n", string(stdout))
	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachment); err != nil {
		fmt.Printf("*** *** The error here is %#v \n\n", err)
		return nil
	}
	return nil

	//var replicationInstanceRecord ReplicationInstanceRecord
	//if err = json.Unmarshal(stdout, &replicationInstanceRecord); err != nil {
	//	fmt.Printf("*** *** The error here is %#v \n\n", err)
	//	return nil
	//}
	//DMSInfo.ReplicationInstanceArn = replicationInstanceRecord.ReplicationInstance.ReplicationInstanceArn
	//for i := 1; i <= 50; i++ {
	//	command = fmt.Sprintf("aws dms describe-replication-instances")
	//	stdout, stderr, err := local.Execute(ctx, command, false)
	///	if err != nil {
	//		fmt.Printf("The error err here is <%#v> \n\n", err)
	//		fmt.Printf("----------\n\n")
	//		fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
	//		return nil
	//	} else {
	//		var replicationInstances ReplicationInstances
	//		if err = json.Unmarshal(stdout, &replicationInstances); err != nil {
	//			fmt.Printf("*** *** The error here is %#v \n\n", err)
	//			return nil
	//		}
	//		fmt.Printf("The db cluster is <%#v> \n\n\n", replicationInstances)
	//		for _, replicationInstance := range replicationInstances.ReplicationInstances {
	//			existsResource := ExistsDMSResource(c.clusterType, c.subClusterType, c.clusterName, replicationInstance.ReplicationInstanceArn, local, ctx)
	//			if existsResource == true {
	//				if replicationInstance.ReplicationInstanceStatus == "available" {
	//					return nil
	//				}
	//			}
	//		}
	//	}

	//	time.Sleep(30 * time.Second)
	//}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
