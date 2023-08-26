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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2utils "github.com/luyomo/OhMyTiUP/pkg/aws/utils/ec2"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type CreateRouteTgw struct {
	pexecutor       *ctxt.Executor
	subClusterType  string
	subClusterTypes []string
	client          *ec2.Client
}

// Execute implements the Task interface
func (c *CreateRouteTgw) Execute(ctx context.Context) error {

	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = clusterName
	mapArgs["clusterType"] = clusterType

	// 01. Get the instance info using AWS SDK
	ec2api, err := ec2utils.NewEC2API(&mapArgs)
	if err != nil {
		return err
	}

	transitGateway, err := ec2api.GetTransitGateway()
	if err != nil {
		return err
	}
	if transitGateway == nil {
		return errors.New("No transit gateway found.")
	}

	mapArgs["subClusterType"] = c.subClusterType

	sourceVpcInfo, err := ec2api.GetVpcId()
	if err != nil {
		return err
	}

	if sourceVpcInfo == nil {
		return errors.New("No source vpc info found.")
	}

	for _, targetSubClusterType := range c.subClusterTypes {

		mapArgs["subClusterType"] = targetSubClusterType
		delete(mapArgs, "scope")

		targetVpcInfo, err := ec2api.GetVpcId()
		if err != nil {
			return err
		}

		if targetVpcInfo == nil {
			return errors.New("No target vpc info found.")
		}

		// Create either of private and public route table to target cidr. Need to improve this logic. It's not easy to understand.
		mapArgs["subClusterType"] = c.subClusterType // It will be used to search the routes
		for _, scope := range []string{"private", "public"} {
			// mapArgs["scope"] = "private"
			mapArgs["scope"] = scope
			if err := ec2api.CreateRoute(*targetVpcInfo.CidrBlock, *transitGateway.TransitGatewayId); err != nil {
				return err
			}
		}

		fmt.Printf("----- Target sub couster type: %s \n\n\n", targetSubClusterType)
		for _, scope := range []string{"private", "public"} {
			mapArgs["subClusterType"] = targetSubClusterType
			// mapArgs["scope"] = "private"
			mapArgs["scope"] = scope
			if err := ec2api.CreateRoute(*sourceVpcInfo.CidrBlock, *transitGateway.TransitGatewayId); err != nil {
				return err
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateRouteTgw) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRouteTgw) String() string {
	return fmt.Sprintf("Echo: Creating route tgw ")
}
