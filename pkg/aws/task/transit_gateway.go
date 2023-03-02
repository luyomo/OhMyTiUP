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
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
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
	pexecutor *ctxt.Executor
}

// create-transit-gateway --description testtisample --tag-specifications ...
// Execute implements the Task interface
func (c *CreateTransitGateway) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	fmt.Printf("Starting to create transit gateway \n\n\n")

	transitGateway, err := getTransitGateway(*c.pexecutor, ctx, clusterName, clusterType)
	if err != nil {
		return err
	}
	if transitGateway != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 create-transit-gateway --description %s --tag-specifications \"ResourceType=transit-gateway,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s}]\"", clusterName, clusterName, clusterType)
	fmt.Printf("command: <%s> \n\n\n\n", command)
	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	for i := 1; i <= 50; i++ {
		transitGateway, err := getTransitGateway(*c.pexecutor, ctx, clusterName, clusterType)
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
	return fmt.Sprintf("Echo: Creating transit gateway ")
}

/******************************************************************************/

type DestroyTransitGateway struct {
	pexecutor *ctxt.Executor
}

func (c *DestroyTransitGateway) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	transitGateway, err := getTransitGateway(*(c.pexecutor), ctx, clusterName, clusterType)

	if err != nil {
		return err
	}
	if transitGateway == nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 delete-transit-gateway --transit-gateway-id %s", transitGateway.TransitGatewayId)
	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
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
	return fmt.Sprintf("Echo: Destryong transit gateway")
}

/******************************************************************************/

type ListTransitGateway struct {
	pexecutor      *ctxt.Executor
	transitGateway *TransitGateway
}

func (c *ListTransitGateway) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	transitGateway, err := getTransitGateway(*(c.pexecutor), ctx, clusterName, clusterType)

	if err != nil {
		return err
	}
	if transitGateway == nil {
		return nil
	}
	*c.transitGateway = *transitGateway
	return nil
}

func (c *ListTransitGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

func (c *ListTransitGateway) String() string {
	return fmt.Sprintf("Echo: Listing transit gateway")
}
