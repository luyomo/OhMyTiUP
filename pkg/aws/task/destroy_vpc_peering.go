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
	//	"time"

	"github.com/luyomo/tisample/pkg/aws/executor"
	//"github.com/luyomo/tisample/pkg/aws/spec"
)

type DestroyVpcPeering struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

// Execute implements the Task interface
func (c *DestroyVpcPeering) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	stdout, stderr, err := local.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpc-peering-connections --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=status-code,Values=failed,expired,provisioning,active,rejected\"", c.clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error here is <%s> \n\n", string(stderr))
		return nil
	}

	var vpcConnections VpcConnections
	if err = json.Unmarshal(stdout, &vpcConnections); err != nil {
		fmt.Printf("The error here is %#v \n\n", err)
		return nil
	}

	for _, pcx := range vpcConnections.VpcPeeringConnections {
		fmt.Printf("The pcx is <%#v> \n\n\n", pcx)
		command := fmt.Sprintf("aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id %s", pcx.VpcPeeringConnectionId)
		stdout, stderr, err = local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n", err)
			fmt.Printf("----------\n\n")
			fmt.Printf("The error here is <%s> \n\n", string(stderr))
			return nil
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
