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
	//	"errors"
	"fmt"
	//"github.com/luyomo/tisample/pkg/aws/spec"
	//	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	//	"go.uber.org/zap"
	//"strings"
	"time"
)

type DestroyTransitGatewayVpcAttachment struct {
	user        string
	host        string
	clusterName string
	clusterType string
}

func (c *DestroyTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})

	command := fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=state,Values=available,modifying,pending\"", c.clusterName, c.clusterType)

	stdout, stderr, err := local.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The comamnd is <%s> \n\n\n", command)
		fmt.Printf("ERRORS: describe-transit-gateway-vpc-attachments  <%s> \n\n", string(stderr))
		return err
	}
	var transitGatewayVpcAttachments TransitGatewayVpcAttachments

	if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
		fmt.Printf("ERRORS: describe-transit-gateway-vpc-attachments json parsing <%s> \n\n", string(stderr))
		return err
	}

	var deletingAttachments []string
	for _, attachment := range transitGatewayVpcAttachments.TransitGatewayVpcAttachments {
		fmt.Printf("The attachment is <%#v> \n\n\n", attachment)
		command = fmt.Sprintf("aws ec2 delete-transit-gateway-vpc-attachment --transit-gateway-attachment-id  %s", attachment.TransitGatewayAttachmentId)
		fmt.Printf("The comamnd is <%s> \n\n\n", command)

		stdout, stderr, err = local.Execute(ctx, command, false)

		if err != nil {
			fmt.Printf("ERRORS: delete-transit-gateway-vpc-attachments json parsing <%s> \n\n", string(stderr))
			return err
		}
		deletingAttachments = append(deletingAttachments, attachment.TransitGatewayAttachmentId)
	}

	command = fmt.Sprintf("aws ec2 describe-transit-gateway-vpc-attachments --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=state,Values=available,modifying,pending,deleting\"", c.clusterName, c.clusterType)
	fmt.Printf("The comamnd is <%s> \n\n\n", command)

	for i := 1; i <= 50; i++ {
		cntAttachments := 0
		stdout, stderr, err := local.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("ERRORS: describe-transit-gateway-vpc-attachments  <%s> \n\n", string(stderr))
			return err
		}
		var transitGatewayVpcAttachments TransitGatewayVpcAttachments

		if err = json.Unmarshal(stdout, &transitGatewayVpcAttachments); err != nil {
			fmt.Printf("ERRORS: describe-transit-gateway-vpc-attachments  <%s> \n\n", string(stderr))
			return err
		}
		for _, attachment := range transitGatewayVpcAttachments.TransitGatewayVpcAttachments {
			for _, hitAttachment := range deletingAttachments {
				if hitAttachment == attachment.TransitGatewayAttachmentId {
					cntAttachments++
				}
			}
		}
		fmt.Printf("The counting here is <%d> \n\n\n", cntAttachments)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
