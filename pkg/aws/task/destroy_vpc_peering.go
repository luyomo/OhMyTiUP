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
	"fmt"
	"go.uber.org/zap"
	"strings"
	//	"time"

	"github.com/luyomo/tisample/pkg/executor"
	//"github.com/luyomo/tisample/pkg/aws/spec"
)

type DestroyVpcPeering struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

func (c *DestroyVpcPeering) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	vpcs, err := getVPCInfos(local, ctx, ResourceTag{clusterName: c.clusterName, clusterType: c.clusterType})
	if err != nil {
		return err
	}

	var arrVpcs []string
	for _, vpc := range (*vpcs).Vpcs {
		arrVpcs = append(arrVpcs, vpc.VpcId)
	}

	command := fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=accepter-vpc-info.vpc-id,Values=%s\" ", strings.Join(arrVpcs, ","))
	fmt.Printf("The command is <%s> \n\n\n", command)

	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var vpcPeerings VPCPeeringConnections
	if err := json.Unmarshal(stdout, &vpcPeerings); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil
	}

	for _, pcx := range vpcPeerings.VpcPeeringConnections {
		fmt.Printf("The vpc info is <%#v> \n\n\n", pcx)
		command := fmt.Sprintf("aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id %s", pcx.VpcPeeringConnectionId)
		_, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("ERRORS: delete-vpc-peering-connection  <%s> \n\n\n", string(stderr))
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyVpcPeering) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyVpcPeering) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
