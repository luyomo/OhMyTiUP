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
	"github.com/luyomo/tisample/pkg/ctxt"
	//	"go.uber.org/zap"
	//"strings"
	//"time"
	"time"
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

type CreateTransitGatewayVpcAttachment struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

func (c *CreateTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=state,Values=available,modifying,pending\"", clusterName, clusterType, c.subClusterType)

	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	var transitGatewayVpcAttachments TransitGatewayVpcAttachments
	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
		return err
	}

	if len(transitGatewayVpcAttachments.TransitGatewayVpcAttachments) > 0 {
		return nil
	}

	transitGateway, err := getTransitGateway(*c.pexecutor, ctx, clusterName)
	if err != nil {
		return err
	}
	if transitGateway == nil {
		return errors.New("No transit gateway found")
	}

	vpc, err := getVPCInfo(*c.pexecutor, ctx, ResourceTag{clusterName: clusterName, clusterType: clusterType, subClusterType: c.subClusterType})
	if err != nil {
		if err.Error() == "No VPC found" {
			return nil
		}
		return err
	}
	if vpc == nil {
		return nil
	}

	subnets, err := getNetworksString(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType, "private")
	if err != nil {
		return err
	}
	if subnets == "" {
		return errors.New("No subnets found")
	}

	command = fmt.Sprintf("aws ec2 create-transit-gateway-vpc-attachment --transit-gateway-id %s --vpc-id %s --subnet-ids '\"'\"'%s'\"'\"' --tag-specifications \"ResourceType=transit-gateway-attachment,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s}]\"", transitGateway.TransitGatewayId, vpc.VpcId, subnets, clusterName, clusterType, c.subClusterType)

	stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	var transitGatewayVpcAttachment TransitGatewayVpcAttachment

	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachment); err != nil {
		return err
	}

	for cnt := 0; cnt < 60; cnt++ {
		command = fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=state,Values=available,modifying,pending\"", clusterName, clusterType, c.subClusterType)

		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
		var transitGatewayVpcAttachments TransitGatewayVpcAttachments
		if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
			return err
		}

		if len(transitGatewayVpcAttachments.TransitGatewayVpcAttachments) > 0 && (transitGatewayVpcAttachments.TransitGatewayVpcAttachments)[0].State == "available" {
			break
		}
		time.Sleep(1 * time.Minute)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: Creating transit gateway vpc attachment")
}

/******************************************************************************/

type DestroyTransitGatewayVpcAttachment struct {
	pexecutor *ctxt.Executor
}

func (c *DestroyTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=state,Values=available,modifying,pending\"", clusterName, clusterType)

	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	var transitGatewayVpcAttachments TransitGatewayVpcAttachments

	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
		return err
	}

	var deletingAttachments []string
	for _, attachment := range transitGatewayVpcAttachments.TransitGatewayVpcAttachments {
		command = fmt.Sprintf("aws ec2 delete-transit-gateway-vpc-attachment --transit-gateway-attachment-id  %s", attachment.TransitGatewayAttachmentId)
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)

		if err != nil {
			return err
		}
		deletingAttachments = append(deletingAttachments, attachment.TransitGatewayAttachmentId)
	}

	command = fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=state,Values=available,modifying,pending,deleting\"", clusterName, clusterType)

	for i := 1; i <= 50; i++ {
		cntAttachments := 0
		stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
		var transitGatewayVpcAttachments TransitGatewayVpcAttachments

		if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
			return err
		}
		for _, attachment := range transitGatewayVpcAttachments.TransitGatewayVpcAttachments {
			for _, hitAttachment := range deletingAttachments {
				if hitAttachment == attachment.TransitGatewayAttachmentId {
					cntAttachments++
				}
			}
		}
		if cntAttachments == 0 {
			break
		}
		time.Sleep(30 * time.Second)
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: Destroying transit gateway vpc attachment")
}

/******************************************************************************/

type ListTransitGatewayVpcAttachment struct {
	pexecutor                         *ctxt.Executor
	tableTransitGatewayVpcAttachments *[][]string
}

func (c *ListTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	command := fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=state,Values=available,modifying,pending\"", clusterName, clusterType)

	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	var transitGatewayVpcAttachments TransitGatewayVpcAttachments

	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
		return err
	}

	for _, attachment := range transitGatewayVpcAttachments.TransitGatewayVpcAttachments {
		componentName := "-"
		for _, tagItem := range attachment.Tags {
			if tagItem.Key == "Type" {
				componentName = tagItem.Value
			}
		}
		(*c.tableTransitGatewayVpcAttachments) = append(*c.tableTransitGatewayVpcAttachments, []string{
			componentName,
			attachment.VpcId,
			attachment.State,
		})
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: Listing transit gateway vpc attachment")
}
