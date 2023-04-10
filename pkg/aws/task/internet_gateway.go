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
func (b *Builder) CreateInternetGateway(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateInternetGateway{BaseInternetGateway: BaseInternetGateway{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}}})
	return b
}

func (b *Builder) ListInternetGateway(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &ListInternetGateway{BaseInternetGateway: BaseInternetGateway{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

func (b *Builder) DestroyInternetGateway(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyInternetGateway{BaseInternetGateway: BaseInternetGateway{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

/******************************************************************************/

type InternetGatewaysInfo struct {
	BaseResourceInfo
}

func (d *InternetGatewaysInfo) ToPrintTable() *[][]string {
	tableInternetGateway := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(InternetGateway)
		// tableInternetGateway = append(tableInternetGateway, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableInternetGateway
}

func (d *InternetGatewaysInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		if throwErr == ThrowErrorIfNotExists {
			return nil, errors.New("No resource found(internet gateway)")
		} else {
			return nil, nil
		}
	}

	return (d.Data[0]).(types.InternetGateway).InternetGatewayId, nil
}

/******************************************************************************/
type BaseInternetGateway struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseInternetGateway) init(ctx context.Context) error {
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
		b.ResourceData = &InternetGatewaysInfo{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseInternetGateway) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeInternetGateways(context.TODO(), &ec2.DescribeInternetGatewaysInput{Filters: *filters})
	if err != nil {
		return err
	}

	for _, internetGateway := range resp.InternetGateways {
		b.ResourceData.Append(internetGateway)
	}

	return nil
}

func (b *BaseInternetGateway) isAttachedToVPC() (bool, error) {
	// TODO: Implement
	resourceExists, err := b.ResourceData.ResourceExist()
	if err != nil {
		return false, err
	}
	if resourceExists == false {
		return false, errors.New("No resource found(internet gateway): Status check")
	}

	_data := b.ResourceData.GetData()

	if len(_data[0].(types.InternetGateway).Attachments) > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

/******************************************************************************/
type CreateInternetGateway struct {
	BaseInternetGateway
}

// Execute implements the Task interface
func (c *CreateInternetGateway) Execute(ctx context.Context) error {
	/* ***** 01. Resource check ******************************************* */
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	/* ***** 02. Create internet gateway if it does not exist ************ */
	if clusterExistFlag == false {
		tags := c.MakeEC2Tags()

		if _, err = c.client.CreateInternetGateway(context.TODO(), &ec2.CreateInternetGatewayInput{
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeSecurityGroup,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}
		c.readResources()
	}

	/* ***** 03. Check the internet gateway status. Return if it is attached to vpc  ************ */
	isAttached, err := c.isAttachedToVPC()
	if err != nil {
		return err
	}

	if isAttached == true {
		return nil
	}
	fmt.Printf("---------- Coming here for attache the internet gateway <%#v> \n\n\n\n\n\n", isAttached)

	/* ***** 04. Attach the internet gateway if it has not been attached any vpc  ************ */
	vpcId, err := c.GetVpcItem("VpcId")
	if err != nil {
		return err
	}

	internetGatewayId, err := c.ResourceData.GetResourceArn(ThrowErrorIfNotExists)
	if err != nil {
		return err
	}

	_, err = c.client.AttachInternetGateway(context.TODO(), &ec2.AttachInternetGatewayInput{
		InternetGatewayId: internetGatewayId,
		VpcId:             vpcId,
	})

	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateInternetGateway) String() string {
	return fmt.Sprintf("Echo: Create InternetGateway ... ...  ")
}

type DestroyInternetGateway struct {
	BaseInternetGateway
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyInternetGateway) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroyInternetGateway ****** \n\n\n")

	internetGateways := c.ResourceData.GetData()

	for _, _entry := range internetGateways {
		internetGateway := _entry.(types.InternetGateway)

		for _, attachment := range internetGateway.Attachments {
			if _, err := c.client.DetachInternetGateway(context.TODO(), &ec2.DetachInternetGatewayInput{
				InternetGatewayId: internetGateway.InternetGatewayId,
				VpcId:             attachment.VpcId,
			}); err != nil {
				return err

			}
		}

		if _, err := c.client.DeleteInternetGateway(context.TODO(), &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: internetGateway.InternetGatewayId,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyInternetGateway) String() string {
	return fmt.Sprintf("Echo: Destroying InternetGateway")
}

type ListInternetGateway struct {
	BaseInternetGateway
}

// Execute implements the Task interface
func (c *ListInternetGateway) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListInternetGateway ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListInternetGateway) String() string {
	return fmt.Sprintf("Echo: List  ")
}
