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

package elb

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	smithy "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
)

func MapTag() *map[string]string {
	return &map[string]string{
		"clusterName":    "Name",
		"clusterType":    "Cluster",
		"subClusterType": "Type",
		"scope":          "Scope",
		"component":      "Component",
	}
}

type ELBAPI struct {
	client *elb.Client

	mapArgs *map[string]string
}

func NewELBAPI(mapArgs *map[string]string) (*ELBAPI, error) {
	elbapi := ELBAPI{}

	if mapArgs != nil {
		elbapi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := elb.NewFromConfig(cfg)
	elbapi.client = client

	return &elbapi, nil
}

func (e *ELBAPI) GetNLB(clusterName string) (*string, error) {
	describeLoadBalancers, err := e.client.DescribeLoadBalancers(context.TODO(), &elb.DescribeLoadBalancersInput{Names: []string{clusterName}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "LoadBalancerNotFound" {
				return nil, nil
			}
		}

		return nil, err
	}
	if len(describeLoadBalancers.LoadBalancers) > 1 {
		return nil, errors.New("Multiple ELB found. ")
	}

	return describeLoadBalancers.LoadBalancers[0].DNSName, nil
}

func (e *ELBAPI) GetNLBInfo(clusterName, componentName string) (*string, *string, *types.LoadBalancerStateEnum, error) {
	describeLoadBalancers, err := e.client.DescribeLoadBalancers(context.TODO(), &elb.DescribeLoadBalancersInput{Names: []string{fmt.Sprintf("%s-%s", clusterName, componentName)}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "LoadBalancerNotFound" {
				return nil, nil, nil, nil
			}
		}

		return nil, nil, nil, err
	}
	return describeLoadBalancers.LoadBalancers[0].LoadBalancerArn, describeLoadBalancers.LoadBalancers[0].DNSName, &describeLoadBalancers.LoadBalancers[0].State.Code, nil
}

func (e *ELBAPI) CreateNLB(clusterName, componentName string, loadBalancerType types.LoadBalancerTypeEnum, subnets *[]string, sg *string) (*string, error) {
	nlbArn, _, _, err := e.GetNLBInfo(clusterName, componentName)
	if err != nil {
		return nil, err
	}
	if nlbArn != nil {
		return nlbArn, nil
	}

	createLoadBalancer, err := e.client.CreateLoadBalancer(context.TODO(), &elb.CreateLoadBalancerInput{
		Name:           aws.String(fmt.Sprintf("%s-%s", clusterName, componentName)),
		Subnets:        *subnets,
		SecurityGroups: []string{*sg},
		Scheme:         types.LoadBalancerSchemeEnumInternal,
		Type:           loadBalancerType, // LoadBalancerTypeEnumApplication
	})
	if err != nil {
		return nil, err
	}
	return createLoadBalancer.LoadBalancers[0].LoadBalancerArn, nil
}

func (e *ELBAPI) DestroyNLB(clusterName, componentName string) error {
	nlbArn, _, _, err := e.GetNLBInfo(clusterName, componentName)
	if err != nil {
		return err
	}
	if nlbArn == nil {
		return nil
	}

	if _, err = e.client.DeleteLoadBalancer(context.TODO(), &elb.DeleteLoadBalancerInput{LoadBalancerArn: nlbArn}); err != nil {
		return err
	}

	return nil
}

func (e *ELBAPI) GetTargetGroupArn(targetGroupName, componentName string) (*string, error) {
	describeTargetGroups, err := e.client.DescribeTargetGroups(context.TODO(), &elb.DescribeTargetGroupsInput{Names: []string{fmt.Sprintf("%s-%s", targetGroupName, componentName)}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			// fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "TargetGroupNotFound" {
				return nil, nil
			}
		}

		return nil, err
	}

	return describeTargetGroups.TargetGroups[0].TargetGroupArn, nil
}

func (e *ELBAPI) CreateTargetGroup(clusterName, componentName string, vpcID *string, port int32, protocolType types.ProtocolEnum, subnets, targetNodes *[]string, sg *string) (*string, error) {
	// 01. Fetch the target group
	targetGroupArn, err := e.GetTargetGroupArn(clusterName, componentName)
	if err != nil {
		return nil, err
	}

	// 02. Create the target group if it does not exist
	if targetGroupArn == nil {
		tags := e.makeTags()
		*tags = append(*tags, types.Tag{Key: aws.String("Component"), Value: aws.String(componentName)})

		fmt.Printf("The tags: %#v \n\n\n", tags)

		resp, err := e.client.CreateTargetGroup(context.TODO(), &elb.CreateTargetGroupInput{
			Name:       aws.String(fmt.Sprintf("%s-%s", clusterName, componentName)),
			Port:       aws.Int32(port),
			Protocol:   protocolType,
			TargetType: types.TargetTypeEnumInstance,
			VpcId:      vpcID,
			Tags:       *tags,
		})
		if err != nil {
			return nil, err
		}

		targetGroupArn = resp.TargetGroups[0].TargetGroupArn
	}

	// 03. Fetch all the targets
	var arrTargets []types.TargetDescription
	for _, instanceId := range *targetNodes {
		arrTargets = append(arrTargets, types.TargetDescription{Id: aws.String(instanceId), Port: aws.Int32(port)})
	}

	// 04. Register the targets to target group
	if _, err = e.client.RegisterTargets(context.TODO(), &elb.RegisterTargetsInput{TargetGroupArn: targetGroupArn, Targets: arrTargets}); err != nil {
		return nil, err
	}

	// 05. If the component is vminsert/vmselect, create the vmendpoint NLB
	if componentName == "vminsert" || componentName == "vmselect" {
		// 05.01 Create vmendpoint NLB
		nlbArn, err := e.CreateNLB(clusterName, "vmendpoint", types.LoadBalancerTypeEnumApplication, subnets, sg)
		if err != nil {
			return nil, err
		}

		// 05.02 Add listener to the NLB
		if _, err = e.client.CreateListener(context.TODO(), &elb.CreateListenerInput{
			LoadBalancerArn: nlbArn,
			Port:            aws.Int32(port),
			Protocol:        protocolType,
			DefaultActions: []types.Action{
				{
					Type:           types.ActionTypeEnumForward,
					TargetGroupArn: targetGroupArn,
				},
			},
		}); err != nil {
			return nil, err
		}
	}

	return targetGroupArn, nil
}

func (e *ELBAPI) DeleteTargetGroup(clusterName, componentName string) error {
	// 01. Fetch the target group
	targetGroupArn, err := e.GetTargetGroupArn(clusterName, componentName)
	if err != nil {
		return err
	}

	// 02. Create the target group if it does not exist
	if targetGroupArn != nil {
		if _, err = e.client.DeleteTargetGroup(context.TODO(), &elb.DeleteTargetGroupInput{TargetGroupArn: targetGroupArn}); err != nil {
			return err
		}
	}

	return nil
}

func (c *ELBAPI) makeTags() *[]types.Tag {
	var tags []types.Tag
	if c.mapArgs == nil {
		return &tags
	}

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			tags = append(tags, types.Tag{Key: aws.String(tagName), Value: aws.String(tagValue)})
		}
	}

	return &tags
}
