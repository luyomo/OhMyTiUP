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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

// type TransitGatewayState_Process types.TransitGatewayState

// func (p TransitGatewayState_Process) isState(mode ReadResourceMode) bool {
// 	switch mode {
// 	case ReadResourceModeCommon:
// 		return p.isOKState()
// 	case ReadResourceModeBeforeCreate:
// 		return p.isBeforeCreateState()
// 	case ReadResourceModeAfterCreate:
// 		return p.isAfterCreateState()
// 	case ReadResourceModeBeforeDestroy:
// 		return p.isBeforeDestroyState()
// 	case ReadResourceModeAfterDestroy:
// 		return p.isAfterDestroyState()
// 	}
// 	return true
// }

// func (p TransitGatewayState_Process) isBeforeCreateState() bool {
// 	return ListContainElement([]string{
// 		string(types.TransitGatewayStatePending),
// 		string(types.TransitGatewayStateAvailable),
// 		string(types.TransitGatewayStateModifying),
// 	}, string(p))

// }

// func (p TransitGatewayState_Process) isAfterCreateState() bool {
// 	return ListContainElement([]string{
// 		string(types.TransitGatewayStateAvailable),
// 	}, string(p))

// }

// func (p TransitGatewayState_Process) isBeforeDestroyState() bool {
// 	return ListContainElement([]string{
// 		string(types.TransitGatewayStatePending),
// 		string(types.TransitGatewayStateAvailable),
// 		string(types.TransitGatewayStateModifying),
// 	}, string(p))

// }

// func (p TransitGatewayState_Process) isAfterDestroyState() bool {
// 	return ListContainElement([]string{
// 		string(types.TransitGatewayStatePending),
// 		string(types.TransitGatewayStateAvailable),
// 		string(types.TransitGatewayStateModifying),
// 		string(types.TransitGatewayStateDeleting),
// 	}, string(p))
// }

// func (p TransitGatewayState_Process) isOKState() bool {
// 	return p.isBeforeCreateState()
// }

/******************************************************************************/
func (b *Builder) CreateAutoScaling(pexecutor *ctxt.Executor, subClusterType, component string, network NetworkType, ec2Node *spec.AwsNodeModal, awsGeneralConfig *spec.AwsTopoConfigsGeneral) *Builder {
	if ec2Node.InstanceType != "" {
		b.tasks = append(b.tasks, &CreateAutoScaling{BaseAutoScaling: BaseAutoScaling{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, component: component, scope: network},
			awsTopoConfigs:    ec2Node,
			awsGeneralConfigs: awsGeneralConfig,
		},
		})
	}
	return b
}

func (b *Builder) ListAutoScaling(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListAutoScaling{
		BaseAutoScaling: BaseAutoScaling{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

func (b *Builder) DestroyAutoScaling(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyAutoScaling{
		BaseAutoScaling: BaseAutoScaling{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

// func (b *Builder) DestroyAutoScalingGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
// 	b.tasks = append(b.tasks, &DestroyAutoScalingGroup{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 	})
// 	return b
// }

/******************************************************************************/

type AutoScalings struct {
	BaseResourceInfo
}

func (d *AutoScalings) ToPrintTable() *[][]string {
	tableAutoScaling := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(AutoScaling)
		// tableAutoScaling = append(tableAutoScaling, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableAutoScaling
}

func (d *AutoScalings) GetResourceArn() (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource found - TODO: replace name")
	}

	return (d.Data[0]).(types.AutoScalingGroup).AutoScalingGroupARN, nil
}

/******************************************************************************/
type BaseAutoScaling struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *autoscaling.Client // Replace the example to specific service

	awsTopoConfigs    *spec.AwsNodeModal
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
}

func (b *BaseAutoScaling) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = autoscaling.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &AutoScalings{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseAutoScaling) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeASFilters()
	if b.awsTopoConfigs != nil && b.awsTopoConfigs.Labels != nil {
		for _, label := range b.awsTopoConfigs.Labels {
			*filters = append(*filters, types.Filter{Name: aws.String("tag:label:" + label.Name), Values: []string{label.Value}})
		}
	}

	resp, err := b.client.DescribeAutoScalingGroups(context.TODO(), &autoscaling.DescribeAutoScalingGroupsInput{Filters: *filters})
	if err != nil {
		return err
	}

	for _, autoScalingGroup := range resp.AutoScalingGroups {
		b.ResourceData.Append(autoScalingGroup)
	}

	return nil

}

/******************************************************************************/
type CreateAutoScaling struct {
	BaseAutoScaling

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateAutoScaling) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {

		tags := c.MakeASTags()
		for _, label := range c.awsTopoConfigs.Labels {
			*tags = append(*tags, types.Tag{Key: aws.String("label:" + label.Name), Value: aws.String(label.Value)})
		}

		templateName := c.makeTemplateName()

		subnets, err := c.GetSubnetsInfo(0)
		if err != nil {
			return err
		}

		createAutoScalingGroupInput := &autoscaling.CreateAutoScalingGroupInput{
			AutoScalingGroupName: aws.String(templateName),
			MaxSize:              aws.Int32(c.awsTopoConfigs.MaxSize),
			MinSize:              aws.Int32(c.awsTopoConfigs.MinSize),
			CapacityRebalance:    aws.Bool(true),
			DesiredCapacity:      aws.Int32(c.awsTopoConfigs.DesiredCapacity),
			LaunchTemplate: &types.LaunchTemplateSpecification{
				LaunchTemplateName: aws.String(templateName),
				Version:            aws.String("$Latest"),
			},
			// VPCZoneIdentifier: aws.String(strings.Join(c.clusterInfo.privateSubnets, ",")),
			VPCZoneIdentifier: aws.String(strings.Join(*subnets, ",")),
			Tags:              *tags,
		}

		// Compatible to the old version of count config value
		if c.awsTopoConfigs.DesiredCapacity == 0 && c.awsTopoConfigs.Count > 0 {
			createAutoScalingGroupInput.MaxSize = aws.Int32(int32(c.awsTopoConfigs.Count))
			createAutoScalingGroupInput.MinSize = aws.Int32(int32(c.awsTopoConfigs.Count))
			createAutoScalingGroupInput.DesiredCapacity = aws.Int32(int32(c.awsTopoConfigs.Count))
		}

		targetGroupArn, err := c.GetTargetGroupArn()
		if err != nil {
			return err
		}
		if targetGroupArn != nil {
			createAutoScalingGroupInput.TargetGroupARNs = []string{*targetGroupArn}
		}

		if _, err := c.client.CreateAutoScalingGroup(context.TODO(), createAutoScalingGroupInput); err != nil {
			return err
		}
	}

	return nil
}

func (c *CreateAutoScaling) makeTemplateName() string {

	var arrLabel []string
	for _, _entry := range c.awsTopoConfigs.Labels {
		arrLabel = append(arrLabel, fmt.Sprintf("%s-%s", _entry.Name, _entry.Value))
	}
	if len(arrLabel) > 0 {
		return fmt.Sprintf("%s.%s.%s.%s.%s", c.clusterType, c.clusterName, c.subClusterType, c.component, strings.Join(arrLabel, "."))
	} else {
		return fmt.Sprintf("%s.%s.%s.%s", c.clusterType, c.clusterName, c.subClusterType, c.component)
	}
}

// Rollback implements the Task interface
func (c *CreateAutoScaling) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateAutoScaling) String() string {
	return fmt.Sprintf("Echo: Create AutoScaling ... ...  ")
}

type DestroyAutoScaling struct {
	BaseAutoScaling
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyAutoScaling) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	for _, _entry := range c.ResourceData.GetData() {
		autoScalingGroup := _entry.(types.AutoScalingGroup)
		if _, err := c.client.DeleteAutoScalingGroup(context.TODO(), &autoscaling.DeleteAutoScalingGroupInput{
			AutoScalingGroupName: autoScalingGroup.AutoScalingGroupName,
			ForceDelete:          aws.Bool(true),
		}); err != nil {
			return err

		}
	}

	if err := c.waitUntilResouceDestroy(0, 0, func() error {
		return c.readResources()
	}); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyAutoScaling) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyAutoScaling) String() string {
	return fmt.Sprintf("Echo: Destroying AutoScaling")
}

type ListAutoScaling struct {
	BaseAutoScaling
}

// Execute implements the Task interface
func (c *ListAutoScaling) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListAutoScaling ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListAutoScaling) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListAutoScaling) String() string {
	return fmt.Sprintf("Echo: List  ")
}
