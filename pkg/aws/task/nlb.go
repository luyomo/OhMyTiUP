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
	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

/******************************************************************************/
func (b *Builder) CreateNLB(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateNLB{BaseNLB: BaseNLB{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypePrivate}}})
	return b
}

func (b *Builder) ListNLB(pexecutor *ctxt.Executor, subClusterType string, nlb *types.LoadBalancer) *Builder {
	b.tasks = append(b.tasks, &ListNLB{BaseNLB: BaseNLB{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}}, nlb: nlb})
	return b
}

func (b *Builder) DestroyNLB(pexecutor *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DestroyNLB{BaseNLB: BaseNLB{BaseTask: BaseTask{pexecutor: pexecutor}}})
	return b
}

/******************************************************************************/

type NLBs struct {
	BaseResourceInfo
}

func (d *NLBs) ToPrintTable() *[][]string {
	tableNLB := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(NLB)
		// tableNLB = append(tableNLB, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableNLB
}

func (d *NLBs) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.LoadBalancer).LoadBalancerArn, nil
	})
}

/******************************************************************************/
type BaseNLB struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *nlb.Client // Replace the example to specific service
}

func (b *BaseNLB) init(ctx context.Context) error {
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
		b.ResourceData = &NLBs{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseNLB) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	describeLoadBalancers, err := b.client.DescribeLoadBalancers(context.TODO(), &nlb.DescribeLoadBalancersInput{Names: []string{b.clusterName}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "LoadBalancerNotFound" {
				return nil
			}
		}

		return err
	}

	for _, loadBalancer := range describeLoadBalancers.LoadBalancers {
		b.ResourceData.Append(loadBalancer)
	}

	return nil
}

/******************************************************************************/
type CreateNLB struct {
	BaseNLB

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateNLB) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		clusterSubnets, err := c.GetSubnetsInfo(0)
		if err != nil {
			return err
		}

		if _, err = c.client.CreateLoadBalancer(context.TODO(), &nlb.CreateLoadBalancerInput{
			Name:    aws.String(c.clusterName),
			Subnets: *clusterSubnets,
			Scheme:  types.LoadBalancerSchemeEnumInternal,
			Type:    types.LoadBalancerTypeEnumNetwork,
		}); err != nil {
			return err
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateNLB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateNLB) String() string {
	return fmt.Sprintf("Echo: Create NLB ... ...  ")
}

type DestroyNLB struct {
	BaseNLB
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyNLB) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	_id, err := c.ResourceData.GetResourceArn(ContinueIfNotExists)
	if err != nil {
		return err
	}

	if _id != nil {
		if _, err = c.client.DeleteLoadBalancer(context.TODO(), &nlb.DeleteLoadBalancerInput{LoadBalancerArn: _id}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyNLB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyNLB) String() string {
	return fmt.Sprintf("Echo: Destroying NLB")
}

type ListNLB struct {
	BaseNLB

	nlb *types.LoadBalancer
}

// Execute implements the Task interface
func (c *ListNLB) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	// The judge must be after init because it has to be initialized.
	if c.nlb == nil {
		return nil
	}

	_data := c.ResourceData.GetData()
	if len(_data) == 0 {
		return nil
	}
	(*c.nlb) = _data[0].(types.LoadBalancer)

	return nil
}

// Rollback implements the Task interface
func (c *ListNLB) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListNLB) String() string {
	return fmt.Sprintf("Echo: List  ")
}

type RegisterTarget struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *RegisterTarget) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	targetGroup, err := getTargetGroup(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
	if err != nil {
		return err
	}

	tidbNodes, err := getEC2Nodes(*c.pexecutor, ctx, clusterName, clusterType, "tidb")
	if err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	clientElb := nlb.NewFromConfig(cfg)

	var arrTargets []types.TargetDescription
	for _, instance := range *tidbNodes {
		arrTargets = append(arrTargets, types.TargetDescription{Id: &instance.InstanceId})
	}

	_, err = clientElb.RegisterTargets(context.TODO(), &nlb.RegisterTargetsInput{TargetGroupArn: targetGroup.TargetGroupArn, Targets: arrTargets})
	if err != nil {
		return err
	}

	return nil
}

func (c *RegisterTarget) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RegisterTarget) String() string {
	return fmt.Sprintf("Echo: Registering Target Group ")
}
