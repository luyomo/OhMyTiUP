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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

type VpcEndpointState_Process string

func (p VpcEndpointState_Process) isState(mode ReadResourceMode) bool {
	switch mode {
	case ReadResourceModeCommon:
		return p.isOKState()
	case ReadResourceModeAfterCreate:
		return p.isAfterCreateState()
	}
	return true
}

func (p VpcEndpointState_Process) isAfterCreateState() bool {
	return ListContainElement([]string{
		string("available"),
		string("pending"),
	}, string(p))

}

func (p VpcEndpointState_Process) isOKState() bool {
	return p == "available"
}

/******************************************************************************/
func (b *Builder) CreateVpcEndpoint(pexecutor *ctxt.Executor, vpceIdChan chan string, subClusterType, component string, network NetworkType, serviceName string) *Builder {
	b.tasks = append(b.tasks, &CreateVpcEndpoint{BaseVpcEndpoint: BaseVpcEndpoint{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, component: component, scope: network},
		serviceName: serviceName,
		vpceIdChan:  vpceIdChan}})

	return b
}

func (b *Builder) ListVpcEndpoint(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListVpcEndpoint{BaseVpcEndpoint: BaseVpcEndpoint{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

func (b *Builder) DestroyVpcEndpoint(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyVpcEndpoint{BaseVpcEndpoint: BaseVpcEndpoint{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

/******************************************************************************/

type VpcEndpointsInfo struct {
	BaseResourceInfo
}

func (d *VpcEndpointsInfo) ToPrintTable() *[][]string {
	tableVpcEndpoint := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(VpcEndpoint)
		// tableVpcEndpoint = append(tableVpcEndpoint, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableVpcEndpoint
}

func (d *VpcEndpointsInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.VpcEndpoint).VpcEndpointId, nil
	})
}

/******************************************************************************/
type BaseVpcEndpoint struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	serviceName string
	vpceIdChan  chan string

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseVpcEndpoint) init(ctx context.Context, mode ReadResourceMode) error {
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
		b.ResourceData = &VpcEndpointsInfo{}
	}

	if err := b.readResources(mode); err != nil {
		return err
	}

	return nil
}

func (b *BaseVpcEndpoint) readResources(mode ReadResourceMode) error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	// TODO: Replace if necessary
	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeVpcEndpoints(context.TODO(), &ec2.DescribeVpcEndpointsInput{
		Filters: *filters,
	})
	if err != nil {
		return err
	}
	fmt.Printf("The response is: <%#v> \n\n\n\n\n\n", resp)

	for _, vpcEndpoint := range resp.VpcEndpoints {
		_state := VpcEndpointState_Process(vpcEndpoint.State)
		if _state.isState(mode) == true {
			b.ResourceData.Append(vpcEndpoint)
		}
	}
	return nil
}

func (b *BaseVpcEndpoint) GetVpcEndpointID() (*string, error) {
	resourceExistFlag, err := b.ResourceData.ResourceExist()
	if err != nil {
		return nil, err
	}

	if resourceExistFlag == false {
		return nil, errors.New("No VpcEndpoint found")
	}

	_data := b.ResourceData.GetData()

	return _data[0].(types.VpcEndpoint).VpcEndpointId, nil

}

/******************************************************************************/
type CreateVpcEndpoint struct {
	BaseVpcEndpoint

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateVpcEndpoint) Execute(ctx context.Context) error {
	fmt.Printf("Starting to create vpc endpoint \n\n\n")

	if err := c.init(ctx, ReadResourceModeAfterCreate); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}
	fmt.Printf("Starting to create resource <%#v> \n\n\n", clusterExistFlag)

	if clusterExistFlag == false {
		// TODO: Add resource preparation
		tags := c.MakeEC2Tags()

		fmt.Printf("CreateRouteTable -> ClusterName: %s, ClusterType: %s, subClusterType: %s, scope: %s \n\n\n\n", c.clusterName, c.clusterType, c.subClusterType, c.scope)
		vpcId, err := c.GetVpcItem("VpcId")
		if err != nil {
			return err
		}

		fmt.Printf("The vpc id is <%s> \n\n\n", *vpcId)

		securityGroup, err := c.GetSecurityGroup(ThrowErrorIfNotExists)
		if err != nil {
			return err
		}
		fmt.Printf("The security group  is <%s> \n\n\n", *securityGroup)

		subnet, err := c.GetSubnetsInfo(1)
		if err != nil {
			return err
		}
		fmt.Printf("The subnet is <%s> \n\n\n", *subnet)

		resp, err := c.client.CreateVpcEndpoint(context.TODO(), &ec2.CreateVpcEndpointInput{
			ServiceName:      aws.String(c.serviceName),
			VpcId:            vpcId,
			SecurityGroupIds: []string{*securityGroup},
			SubnetIds:        *subnet,
			VpcEndpointType:  types.VpcEndpointTypeInterface,

			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeVpcEndpoint,
					Tags:         *tags,
				},
			},
		})
		if err != nil {
			return err
		}

		c.vpceIdChan <- *resp.VpcEndpoint.VpcEndpointId

		if err := c.waitUntilResouceAvailable(0, 0, 1, func() error {
			return c.readResources(ReadResourceModeCommon)
		}); err != nil {
			return err
		}

		_, err = c.client.ModifyVpcEndpoint(context.TODO(), &ec2.ModifyVpcEndpointInput{
			VpcEndpointId:     resp.VpcEndpoint.VpcEndpointId,
			PrivateDnsEnabled: aws.Bool(true),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateVpcEndpoint) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateVpcEndpoint) String() string {
	return fmt.Sprintf("Echo: Create VpcEndpoint ... ...  ")
}

type DestroyVpcEndpoint struct {
	BaseVpcEndpoint
}

// Execute implements the Task interface
func (c *DestroyVpcEndpoint) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeBeforeDestroy) // ClusterName/ClusterType and client initialization

	_data := c.ResourceData.GetData()
	for _, vpcEndpoint := range _data {
		_entry := vpcEndpoint.(types.VpcEndpoint)
		if _, err := c.client.DeleteVpcEndpoints(context.TODO(), &ec2.DeleteVpcEndpointsInput{
			VpcEndpointIds: []string{*_entry.VpcEndpointId},
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
func (c *DestroyVpcEndpoint) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyVpcEndpoint) String() string {
	return fmt.Sprintf("Echo: Destroying VpcEndpoint")
}

type ListVpcEndpoint struct {
	BaseVpcEndpoint

	vpcEndpoint *types.VpcEndpoint
}

// Execute implements the Task interface
func (c *ListVpcEndpoint) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeCommon) // ClusterName/ClusterType and client initialization
	return nil
}

// Rollback implements the Task interface
func (c *ListVpcEndpoint) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListVpcEndpoint) String() string {
	return fmt.Sprintf("Echo: List  ")
}