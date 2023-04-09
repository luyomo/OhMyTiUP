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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateLaunchTemplate(pexecutor *ctxt.Executor, subClusterType, component string, network NetworkType, ec2Node *spec.AwsNodeModal, awsGeneralConfig *spec.AwsTopoConfigsGeneral) *Builder {
	if ec2Node.InstanceType != "" {
		b.tasks = append(b.tasks, &CreateLaunchTemplate{BaseLaunchTemplate: BaseLaunchTemplate{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, component: component, scope: network},
			awsTopoConfigs:    ec2Node,
			awsGeneralConfigs: awsGeneralConfig},
		})
	}
	return b
}

func (b *Builder) ListLaunchTemplate(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListLaunchTemplate{
		BaseLaunchTemplate: BaseLaunchTemplate{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

// func (b *Builder) DestroyLaunchTemplate(pexecutor *ctxt.Executor, subClusterType string) *Builder {
// 	b.tasks = append(b.tasks, &DestroyLaunchTemplate{
// 		pexecutor:      pexecutor,
// 		subClusterType: subClusterType,
// 	})
// 	return b
// }

func (b *Builder) DestroyLaunchTemplate(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyLaunchTemplate{
		BaseLaunchTemplate: BaseLaunchTemplate{BaseTask: BaseTask{pexecutor: pexecutor}},
	})
	return b
}

/******************************************************************************/

type LaunchTemplates struct {
	BaseResourceInfo
}

func (d *LaunchTemplates) ToPrintTable() *[][]string {
	tableLaunchTemplate := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(LaunchTemplate)
		// tableLaunchTemplate = append(tableLaunchTemplate, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableLaunchTemplate
}

func (d *LaunchTemplates) GetResourceArn() (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource found - TODO: replace name")
	}

	return (d.Data[0]).(types.LaunchTemplate).LaunchTemplateId, nil
}

/******************************************************************************/
type BaseLaunchTemplate struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service

	awsTopoConfigs    *spec.AwsNodeModal
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
}

func (b *BaseLaunchTemplate) init(ctx context.Context) error {
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
		b.ResourceData = &LaunchTemplates{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseLaunchTemplate) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeEC2Filters()

	if b.awsTopoConfigs != nil && b.awsTopoConfigs.Labels != nil {
		for _, label := range b.awsTopoConfigs.Labels {
			*filters = append(*filters, types.Filter{Name: aws.String("tag:label:" + label.Name), Values: []string{label.Value}})
		}
	}

	resp, err := b.client.DescribeLaunchTemplates(context.TODO(), &ec2.DescribeLaunchTemplatesInput{Filters: *filters})
	if err != nil {
		return err
	}

	for _, launchTemplate := range resp.LaunchTemplates {
		b.ResourceData.Append(launchTemplate)
	}

	return nil
}

/******************************************************************************/
type CreateLaunchTemplate struct {
	BaseLaunchTemplate

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateLaunchTemplate) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		securityGroupID, err := c.GetSecurityGroup()
		if err != nil {
			return err
		}

		requestLaunchTemplateData := types.RequestLaunchTemplateData{}

		// Setup the root device
		var launchTemplateBlockDeviceMappingRequest []types.LaunchTemplateBlockDeviceMappingRequest
		rootBlockDeviceMapping := types.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &types.LaunchTemplateEbsBlockDeviceRequest{
				DeleteOnTermination: aws.Bool(true),
				VolumeSize:          aws.Int32(8),
				VolumeType:          types.VolumeType("gp2"),
			},
		}
		launchTemplateBlockDeviceMappingRequest = append(launchTemplateBlockDeviceMappingRequest, rootBlockDeviceMapping)

		// Setup the instance device
		if c.awsTopoConfigs.VolumeType != "" {
			blockDeviceMapping := types.LaunchTemplateBlockDeviceMappingRequest{
				DeviceName: aws.String("/dev/sdb"),
				Ebs: &types.LaunchTemplateEbsBlockDeviceRequest{
					DeleteOnTermination: aws.Bool(true),
					Iops:                aws.Int32(int32(c.awsTopoConfigs.Iops)),
					VolumeSize:          aws.Int32(int32(c.awsTopoConfigs.VolumeSize)),
					VolumeType:          types.VolumeType(c.awsTopoConfigs.VolumeType),
				},
			}

			launchTemplateBlockDeviceMappingRequest = append(launchTemplateBlockDeviceMappingRequest, blockDeviceMapping)
		}
		requestLaunchTemplateData.BlockDeviceMappings = launchTemplateBlockDeviceMappingRequest
		requestLaunchTemplateData.EbsOptimized = aws.Bool(false)                                   // EbsOptimized flag, not support all the instance type
		requestLaunchTemplateData.ImageId = aws.String(c.awsGeneralConfigs.ImageId)                // ImageID
		requestLaunchTemplateData.InstanceType = types.InstanceType(c.awsTopoConfigs.InstanceType) // Instance Type
		requestLaunchTemplateData.KeyName = aws.String(c.awsGeneralConfigs.KeyName)                // Key name
		// requestLaunchTemplateData.SecurityGroupIds = []string{c.clusterInfo.privateSecurityGroupId} // security group
		requestLaunchTemplateData.SecurityGroupIds = []string{*securityGroupID} // security group

		// 03. Template data preparation
		tags := c.MakeEC2Tags()
		for _, label := range c.awsTopoConfigs.Labels {
			*tags = append(*tags, types.Tag{Key: aws.String("label:" + label.Name), Value: aws.String(label.Value)})
		}

		templateName := c.makeTemplateName()
		createLaunchTemplateInput := &ec2.CreateLaunchTemplateInput{
			LaunchTemplateName: aws.String(templateName),
			LaunchTemplateData: &requestLaunchTemplateData,
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{ResourceType: types.ResourceTypeLaunchTemplate, Tags: *tags},
			},
		}

		// 04. Template generation
		if _, err := c.client.CreateLaunchTemplate(context.TODO(), createLaunchTemplateInput); err != nil {
			return errors.New(fmt.Sprintf("%s - %s", err.Error(), templateName))
		}

	}

	return nil
}

func (c *CreateLaunchTemplate) makeTemplateName() string {

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
func (c *CreateLaunchTemplate) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateLaunchTemplate) String() string {
	return fmt.Sprintf("Echo: Create LaunchTemplate ... ...  ")
}

type DestroyLaunchTemplate struct {
	BaseLaunchTemplate
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyLaunchTemplate) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	for _, _entry := range c.ResourceData.GetData() {
		launchTemplate := _entry.(types.LaunchTemplate)
		if _, err := c.client.DeleteLaunchTemplate(context.TODO(), &ec2.DeleteLaunchTemplateInput{LaunchTemplateId: launchTemplate.LaunchTemplateId}); err != nil {
			return err

		}
	}

	// TODO: Destroy the cluster

	// if _, err = c.client.CreateRouteTable(context.TODO(), &ec2.CreateRouteTableInput{
	// 	RouteTableId: _id,
	// }); err != nil {
	// 	return err
	// }

	return nil
}

// Rollback implements the Task interface
func (c *DestroyLaunchTemplate) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyLaunchTemplate) String() string {
	return fmt.Sprintf("Echo: Destroying LaunchTemplate")
}

type ListLaunchTemplate struct {
	BaseLaunchTemplate
}

// Execute implements the Task interface
func (c *ListLaunchTemplate) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListLaunchTemplate ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListLaunchTemplate) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListLaunchTemplate) String() string {
	return fmt.Sprintf("Echo: List  ")
}
