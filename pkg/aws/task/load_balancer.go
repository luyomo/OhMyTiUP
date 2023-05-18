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
	// "github.com/aws/smithy-go"

	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	// "go.uber.org/zap"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

/******************************************************************************/

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

	clientElb := elb.NewFromConfig(cfg)

	var arrTargets []elbtypes.TargetDescription
	for _, instance := range *tidbNodes {
		arrTargets = append(arrTargets, elbtypes.TargetDescription{Id: &instance.InstanceId})
	}

	_, err = clientElb.RegisterTargets(context.TODO(), &elb.RegisterTargetsInput{TargetGroupArn: targetGroup.TargetGroupArn, Targets: arrTargets})
	if err != nil {
		return err
	}

	return nil

	var arrInstance []string
	for _, instance := range *tidbNodes {
		arrInstance = append(arrInstance, fmt.Sprintf("Id=%s", instance.InstanceId))
	}

	fmt.Printf("The instance are <%#v> \n\n\n\n", arrInstance)
	fmt.Printf("The target group arn is <%#v> \n\n\n\n", targetGroup)

	command := fmt.Sprintf("aws elbv2 register-targets --target-group-arn %s --targets %s ", *targetGroup.TargetGroupArn, strings.Join(arrInstance, " "))
	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	return nil

	// aws elbv2 register-targets --target-group-arn arn:aws:elasticloadbalancing:ap-northeast-1:385595570414:targetgroup/testtisample/b446a8cd70efca38 --targets Id=i-00f8c695d756bf19e Id=i-074aed5319afd0c5e
	// 1. Fetch the target group arn
	// 2. Get all the instance id

	return nil
}

func (c *RegisterTarget) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RegisterTarget) String() string {
	return fmt.Sprintf("Echo: Creating Target Group ")
}

/******************************************************************************/

// type CreateNLB struct {
// 	pexecutor      *ctxt.Executor
// 	subClusterType string
// 	clusterInfo    *ClusterInfo
// }

// // Execute implements the Task interface
// func (c *CreateNLB) Execute(ctx context.Context) error {
// 	clusterName := ctx.Value("clusterName").(string)
// 	clusterType := ctx.Value("clusterType").(string)

// 	subnets, err := getNetworks(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType, "private")
// 	if err != nil {
// 		return err
// 	}

// 	var arrSubnets []string
// 	for _, subnet := range *subnets {
// 		arrSubnets = append(arrSubnets, subnet.SubnetId)
// 	}

// 	command := fmt.Sprintf("aws elbv2 create-load-balancer --name %s --type network --subnets %s --scheme internal --tags Key=Name,Value=%s Key=Cluster,Value=%s Key=Type,Value=%s", clusterName, strings.Join(arrSubnets, " "), clusterName, clusterType, c.subClusterType)
// 	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// 	// aws elbv2 create-load-balancer --name testtisample  --type network --subnets subnet-008d0786c982a0888 subnet-04f7f8418f6cf18d8 subnet-031e5e77fa0a5a975 --scheme internal
// 	// 1. Get all the subnets

// }

// func (c *CreateNLB) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// // String implements the fmt.Stringer interface
// func (c *CreateNLB) String() string {
// 	return fmt.Sprintf("Echo: Creating Target Group ")
// }

/******************************************************************************/

// type CreateNLBListener struct {
// 	pexecutor      *ctxt.Executor
// 	subClusterType string
// 	clusterInfo    *ClusterInfo
// }

// // Execute implements the Task interface
// func (c *CreateNLBListener) Execute(ctx context.Context) error {
// 	clusterName := ctx.Value("clusterName").(string)
// 	clusterType := ctx.Value("clusterType").(string)

// 	targetGroup, err := getTargetGroup(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
// 	if err != nil {
// 		return err
// 	}

// 	nlb, err := getNLB(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
// 	if err != nil {
// 		return err
// 	}

// 	command := fmt.Sprintf("aws elbv2 create-listener --load-balancer-arn %s --protocol TCP --port 4000 --default-actions Type=forward,TargetGroupArn=%s", *nlb.LoadBalancerArn, *targetGroup.TargetGroupArn)
// 	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
// 	if err != nil {
// 		return err
// 	}

// 	// aws elbv2 create-listener --load-balancer-arn arn:aws:elasticloadbalancing:ap-northeast-1:385595570414:loadbalancer/net/testtisample/228507d53f1808b9 --protocol TCP --port 4000 --default-actions Type=forward,TargetGroupArn=arn:aws:elasticloadbalancing:ap-northeast-1:385595570414:targetgroup/testtisample/b446a8cd70efca38
// 	// 1. Get load balancer arn
// 	// 2. Get target group arn

// 	return nil
// }

// func (c *CreateNLBListener) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// // String implements the fmt.Stringer interface
// func (c *CreateNLBListener) String() string {
// 	return fmt.Sprintf("Echo: Creating Target Group ")
// }

/******************************************************************************/

// type DestroyNLB struct {
// 	pexecutor      *ctxt.Executor
// 	subClusterType string
// }

// // Execute implements the Task interface
// func (c *DestroyNLB) Execute(ctx context.Context) error {
// 	clusterName := ctx.Value("clusterName").(string)
// 	clusterType := ctx.Value("clusterType").(string)

// 	nlb, err := getNLB(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
// 	if err != nil {
// 		return err
// 	}

// 	if nlb == nil {
// 		return nil
// 	}

// 	command := fmt.Sprintf("aws elbv2 delete-load-balancer --load-balancer-arn %s", *(*nlb).LoadBalancerArn)
// 	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (c *DestroyNLB) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// // String implements the fmt.Stringer interface
// func (c *DestroyNLB) String() string {
// 	return fmt.Sprintf("Echo: Destroying NLB ")
// }

/******************************************************************************/

// type DestroyTargetGroup struct {
// 	pexecutor      *ctxt.Executor
// 	subClusterType string
// }

// // Execute implements the Task interface
// func (c *DestroyTargetGroup) Execute(ctx context.Context) error {
// 	clusterName := ctx.Value("clusterName").(string)
// 	clusterType := ctx.Value("clusterType").(string)

// 	targetGroup, err := getTargetGroup(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
// 	if err != nil {
// 		return err
// 	}
// 	if targetGroup == nil {
// 		return nil
// 	}

// 	command := fmt.Sprintf("aws elbv2 delete-target-group --target-group-arn %s", *(*targetGroup).TargetGroupArn)
// 	_, _, err = (*c.pexecutor).Execute(ctx, command, false)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (c *DestroyTargetGroup) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// // String implements the fmt.Stringer interface
// func (c *DestroyTargetGroup) String() string {
// 	return fmt.Sprintf("Echo: Destroying Target Group ")
// }

/******************************************************************************/

// type ListNLB struct {
// 	pexecutor      *ctxt.Executor
// 	subClusterType string
// 	nlb            *elbtypes.LoadBalancer
// }

// // Execute implements the Task interface
// func (c *ListNLB) Execute(ctx context.Context) error {
// 	clusterName := ctx.Value("clusterName").(string)
// 	clusterType := ctx.Value("clusterType").(string)

// 	nlb, err := getNLB(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
// 	if err != nil {
// 		return err
// 	}
// 	c.nlb = nlb

// 	return nil
// }

// // Rollback implements the Task interface
// func (c *ListNLB) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// // String implements the fmt.Stringer interface
// func (c *ListNLB) String() string {
// 	return fmt.Sprintf("Echo: Listing Load Balancer")
// }
