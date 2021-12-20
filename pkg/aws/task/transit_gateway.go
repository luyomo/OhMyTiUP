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
	"errors"
	"fmt"
	"github.com/luyomo/tisample/pkg/executor"
	"time"
)

type TransitGateway struct {
	TransitGatewayId  string `json:"TransitGatewayId"`
	TransitGatewayArn string `json:"TransitGatewayArn`
	State             string `json:"State"`
}

type TransitGateways struct {
	TransitGateways []TransitGateway `json:"TransitGateways"`
}

type CreateTransitGateway struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

//
// create-transit-gateway --description testtisample --tag-specifications ...
// Execute implements the Task interface
func (c *CreateTransitGateway) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	transitGateway, err := getTransitGateway(local, ctx, c.clusterName)
	if err != nil {
		return err
	}
	if transitGateway != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-transit-gateway --description %s --tag-specifications --tag-specifications \"ResourceType=transit-gateway,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s}]\"", c.clusterName, c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	_, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n\n", err)
		fmt.Printf("The error here is <%s> \n\n\n", string(stderr))
		return err
	}

	for i := 1; i <= 50; i++ {
		transitGateway, err := getTransitGateway(local, ctx, c.clusterName)
		if err != nil {
			return err
		}
		if transitGateway == nil {
			return errors.New("No transit gateway found")
		}
		if transitGateway.State == "available" {
			break
		}

		time.Sleep(30 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTransitGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTransitGateway) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type DestroyTransitGateway struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

func (c *DestroyTransitGateway) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	transitGateway, err := getTransitGateway(local, ctx, c.clusterName)

	if err != nil {
		return err
	}
	if transitGateway == nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 delete-transit-gateway --transit-gateway-id %s", transitGateway.TransitGatewayId)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)
	_, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("ERRORS: delete-transit-gateway <%s> \n\n\n", string(stderr))
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTransitGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTransitGateway) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
