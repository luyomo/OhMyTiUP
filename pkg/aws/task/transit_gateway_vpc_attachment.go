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

	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/

// Before start, check the attachment whose value match some values
// After start, wait the state to become available

// Before destroy, check the attachment whose value match some values
// After destroy, check the attachment whose value is same to before destroy

type TransitGatewayAttachmentState_Process types.TransitGatewayAttachmentState

func (p TransitGatewayAttachmentState_Process) isState(mode ReadResourceMode) bool {
	switch mode {
	case ReadResourceModeCommon:
		return p.isOKState()
	case ReadResourceModeBeforeCreate:
		return p.isBeforeCreateState()
	case ReadResourceModeAfterCreate:
		return p.isAfterCreateState()
	case ReadResourceModeBeforeDestroy:
		return p.isBeforeDestroyState()
	case ReadResourceModeAfterDestroy:
		return p.isAfterDestroyState()
	}
	return true
}

func (p TransitGatewayAttachmentState_Process) isBeforeCreateState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayAttachmentStateInitiating),
		string(types.TransitGatewayAttachmentStateInitiatingRequest),
		string(types.TransitGatewayAttachmentStatePending),
		string(types.TransitGatewayAttachmentStateAvailable),
		string(types.TransitGatewayAttachmentStateModifying),
		string(types.TransitGatewayAttachmentStateFailed),
		string(types.TransitGatewayAttachmentStateFailing),
	}, string(p))

}

func (p TransitGatewayAttachmentState_Process) isAfterCreateState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayAttachmentStateAvailable),
	}, string(p))

}

func (p TransitGatewayAttachmentState_Process) isBeforeDestroyState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayAttachmentStateInitiating),
		string(types.TransitGatewayAttachmentStateInitiatingRequest),
		string(types.TransitGatewayAttachmentStatePending),
		string(types.TransitGatewayAttachmentStateAvailable),
		string(types.TransitGatewayAttachmentStateModifying),
		string(types.TransitGatewayAttachmentStateFailed),
		string(types.TransitGatewayAttachmentStateFailing),
	}, string(p))

}

func (p TransitGatewayAttachmentState_Process) isAfterDestroyState() bool {
	return ListContainElement([]string{
		string(types.TransitGatewayAttachmentStateInitiating),
		string(types.TransitGatewayAttachmentStateInitiatingRequest),
		string(types.TransitGatewayAttachmentStatePending),
		string(types.TransitGatewayAttachmentStateAvailable),
		string(types.TransitGatewayAttachmentStateModifying),
		string(types.TransitGatewayAttachmentStateFailed),
		string(types.TransitGatewayAttachmentStateFailing),
		string(types.TransitGatewayAttachmentStateDeleting),
	}, string(p))
}

func (p TransitGatewayAttachmentState_Process) isOKState() bool {
	return p.isBeforeCreateState()
}

/* **************************************************** */

func (b *Builder) CreateTransitGatewayVpcAttachment(pexecutor *ctxt.Executor, subClusterType string, network NetworkType) *Builder {
	var scope NetworkType
	// If the network type is nat, it is not included into transit gateway attachment.
	if network == NetworkTypeNAT {
		scope = NetworkTypePrivate
	} else {
		scope = network
	}
	b.tasks = append(b.tasks, &CreateTransitGatewayVpcAttachment{
		BaseTransitGatewayVpcAttachment: BaseTransitGatewayVpcAttachment{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: scope}},
	})
	return b
}

func (b *Builder) ListTransitGatewayVpcAttachment(pexecutor *ctxt.Executor, tableTransitGatewayVpcAttachments *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListTransitGatewayVpcAttachment{
		BaseTransitGatewayVpcAttachment:   BaseTransitGatewayVpcAttachment{BaseTask: BaseTask{pexecutor: pexecutor}},
		tableTransitGatewayVpcAttachments: tableTransitGatewayVpcAttachments,
	})
	return b
}

func (b *Builder) DestroyTransitGatewayVpcAttachment(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyTransitGatewayVpcAttachment{
		BaseTransitGatewayVpcAttachment: BaseTransitGatewayVpcAttachment{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

//func (b *Builder) CreateTransitGatewayVpcAttachment() *Builder {
// 	b.tasks = append(b.tasks, &CreateTransitGatewayVpcAttachment{})
// 	return b
// }

// func (b *Builder) ListTransitGatewayVpcAttachment() *Builder {
// 	b.tasks = append(b.tasks, &ListTransitGatewayVpcAttachment{})
// 	return b
// }

// func (b *Builder) DestroyTransitGatewayVpcAttachment() *Builder {
// 	b.tasks = append(b.tasks, &DestroyTransitGatewayVpcAttachment{})
// 	return b
// }

/******************************************************************************/

type TransitGatewayVpcAttachments struct {
	BaseResourceInfo
}

func (d *TransitGatewayVpcAttachments) ToPrintTable() *[][]string {
	tableTransitGatewayVpcAttachment := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(TransitGatewayVpcAttachment)
		// tableTransitGatewayVpcAttachment = append(tableTransitGatewayVpcAttachment, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableTransitGatewayVpcAttachment
}

func (d *TransitGatewayVpcAttachments) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.TransitGatewayVpcAttachment).TransitGatewayAttachmentId, nil
	})
}

/******************************************************************************/
type BaseTransitGatewayVpcAttachment struct {
	BaseTask

	// ResourceData ResourceData
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseTransitGatewayVpcAttachment) init(ctx context.Context, mode ReadResourceMode) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = ec2.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &TransitGatewayVpcAttachments{}
	}

	if err := b.readResources(mode); err != nil {
		return err
	}

	return nil
}

func (b *BaseTransitGatewayVpcAttachment) readResources(mode ReadResourceMode) error {

	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeTransitGatewayAttachments(context.TODO(), &ec2.DescribeTransitGatewayAttachmentsInput{
		Filters: *filters,
	})
	if err != nil {
		return err
	}

	for _, transitGatewayAttachment := range resp.TransitGatewayAttachments {
		_state := TransitGatewayAttachmentState_Process(transitGatewayAttachment.State)
		if _state.isState(mode) == true {
			b.ResourceData.Append(transitGatewayAttachment)
		}
	}
	return nil
}

/******************************************************************************/
type CreateTransitGatewayVpcAttachment struct {
	BaseTransitGatewayVpcAttachment

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	if err := c.init(ctx, ReadResourceModeAfterCreate); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		// TODO: Add resource preparation

		tags := c.MakeEC2Tags()

		clusterSubnets, err := c.GetSubnetsInfo(0)
		if err != nil {
			return err
		}

		vpcId, err := c.GetVpcItem("VpcId")
		if err != nil {
			return err
		}

		transitGatewayID, err := c.GetTransitGatewayID()
		if err != nil {
			return err
		}

		if _, err = c.client.CreateTransitGatewayVpcAttachment(context.TODO(), &ec2.CreateTransitGatewayVpcAttachmentInput{
			VpcId:            vpcId,
			SubnetIds:        *clusterSubnets,
			TransitGatewayId: transitGatewayID,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeTransitGatewayAttachment,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}

		if err := c.waitUntilResouceAvailable(0, 0, 1, func() error {
			return c.readResources(ReadResourceModeAfterCreate)
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: Create TransitGatewayVpcAttachment ... ...  ")
}

type DestroyTransitGatewayVpcAttachment struct {
	BaseTransitGatewayVpcAttachment
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	if err := c.init(ctx, ReadResourceModeBeforeDestroy); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	_data := c.ResourceData.GetData()
	for _, attachment := range _data {
		_entry := attachment.(types.TransitGatewayAttachment)
		if _, err := c.client.DeleteTransitGatewayVpcAttachment(context.TODO(), &ec2.DeleteTransitGatewayVpcAttachmentInput{
			TransitGatewayAttachmentId: _entry.TransitGatewayAttachmentId,
		}); err != nil {
			return err
		}
	}

	if err := c.waitUntilResouceDestroy(0, 0, func() error {
		return c.readResources(ReadResourceModeAfterDestroy)
	}); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: Destroying TransitGatewayVpcAttachment")
}

type ListTransitGatewayVpcAttachment struct {
	BaseTransitGatewayVpcAttachment

	tableTransitGatewayVpcAttachments *[][]string
}

// Execute implements the Task interface
func (c *ListTransitGatewayVpcAttachment) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeCommon) // ClusterName/ClusterType and client initialization

	return nil
}

// Rollback implements the Task interface
func (c *ListTransitGatewayVpcAttachment) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTransitGatewayVpcAttachment) String() string {
	return fmt.Sprintf("Echo: List  ")
}
