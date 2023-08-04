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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	nlb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
// func (b *Builder) CreateTargetGroup(pexecutor *ctxt.Executor, subClusterType string, clusterInfo *ClusterInfo) *Builder {
// 	b.tasks = append(b.tasks, &CreateTargetGroup{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 		clusterInfo:    clusterInfo,
// 	})
// 	return b
// }

// func (b *Builder) DestroyTargetGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
// 	b.tasks = append(b.tasks, &DestroyTargetGroup{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 	})
// 	return b
// }

func (b *Builder) CreateTargetGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateTargetGroup{BaseTargetGroup: BaseTargetGroup{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypePrivate}}})
	return b
}

func (b *Builder) ListTargetGroup(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListTargetGroup{BaseTargetGroup: BaseTargetGroup{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

func (b *Builder) DestroyTargetGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyTargetGroup{BaseTargetGroup: BaseTargetGroup{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}}})
	return b
}

/******************************************************************************/

type TargetGroups struct {
	BaseResourceInfo
}

func (d *TargetGroups) ToPrintTable() *[][]string {
	tableTargetGroup := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(TargetGroup)
		// tableTargetGroup = append(tableTargetGroup, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableTargetGroup
}

func (d *TargetGroups) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.TargetGroup).TargetGroupArn, nil
	})
}

/******************************************************************************/
type BaseTargetGroup struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *nlb.Client // Replace the example to specific service
}

func (b *BaseTargetGroup) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = nlb.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &TargetGroups{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseTargetGroup) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	resp, err := b.client.DescribeTargetGroups(context.TODO(), &nlb.DescribeTargetGroupsInput{Names: []string{b.clusterName}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			// fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "TargetGroupNotFound" {
				return nil
			}
		}

		return err

	}

	for _, targetGroup := range resp.TargetGroups {
		b.ResourceData.Append(targetGroup)
	}
	return nil
}

/******************************************************************************/
type CreateTargetGroup struct {
	BaseTargetGroup

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTargetGroup) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		tags := c.MakeNLBTags()
		vpcId, err := c.GetVpcItem("VpcId")
		if err != nil {
			return err
		}

		if _, err = c.client.CreateTargetGroup(context.TODO(), &nlb.CreateTargetGroupInput{
			Name:       aws.String(c.clusterName),
			Port:       aws.Int32(4000),
			Protocol:   types.ProtocolEnumTcp,
			TargetType: types.TargetTypeEnumInstance,
			VpcId:      vpcId,
			Tags:       *tags,
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateTargetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTargetGroup) String() string {
	return fmt.Sprintf("Echo: Create TargetGroup ... ...  ")
}

type DestroyTargetGroup struct {
	BaseTargetGroup
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyTargetGroup) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	_id, err := c.ResourceData.GetResourceArn(ContinueIfNotExists)
	if err != nil {
		return err
	}

	if _id != nil {
		// TODO: Destroy the cluster

		if _, err = c.client.DeleteTargetGroup(context.TODO(), &nlb.DeleteTargetGroupInput{
			TargetGroupArn: _id,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyTargetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyTargetGroup) String() string {
	return fmt.Sprintf("Echo: Destroying TargetGroup")
}

type ListTargetGroup struct {
	BaseTargetGroup
}

// Execute implements the Task interface
func (c *ListTargetGroup) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListTargetGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListTargetGroup) String() string {
	return fmt.Sprintf("Echo: List  ")
}
