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
	//	"encoding/json"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
	"strings"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"strings"
)

type Vpc struct {
	CidrBlock string `json:"CidrBlock"`
	State     string `json:"State"`
	VpcId     string `json:"VpcId"`
	OwnerId   string `json:"OwnerId"`
}

type Vpcs struct {
	Vpcs []Vpc `json:"Vpcs"`
}

type ClusterInfo struct {
	cidr                   string
	region                 string
	keyName                string
	instanceType           string
	vpcInfo                Vpc
	privateRouteTableId    string
	publicRouteTableId     string
	privateSecurityGroupId string
	publicSecurityGroupId  string
	privateSubnets         []string
	publicSubnet           string
	pcxTidb2Aurora         string
}

func (v Vpc) String() string {
	return fmt.Sprintf("Cidr: %s, State: %s, VpcId: %s, OwnerId: %s", v.CidrBlock, v.State, v.VpcId, v.OwnerId)
}

func (c ClusterInfo) String() string {
	return fmt.Sprintf("vpcInfo:[%s], privateRouteTableId:%s, publicRouteTableId:%s, privateSecurityGroupId:%s, publicSecurityGroupId:%s, privateSubnets:%s, publicSubnet:%s, pcxTidb2Aurora:%s", c.vpcInfo.String(), c.privateRouteTableId, c.publicRouteTableId, c.privateSecurityGroupId, c.publicSecurityGroupId, strings.Join(c.privateSubnets, ","), c.publicSubnet, c.pcxTidb2Aurora)
}

type RouteTable struct {
	RouteTableId string `json:"RouteTableId"`
}

type ResultRouteTable struct {
	TheRouteTable RouteTable `json:"RouteTable"`
}

type RouteTables struct {
	RouteTables []RouteTable `json:"RouteTables"`
}

func (r RouteTable) String() string {
	return fmt.Sprintf("RouteTableId:%s", r.RouteTableId)
}

func (r ResultRouteTable) String() string {
	return fmt.Sprintf("RetRouteTable:%s", r.String())
}

func (r RouteTables) String() string {
	var res []string
	for _, route := range r.RouteTables {
		res = append(res, route.String())
	}
	return fmt.Sprintf("RouteTables:%s", strings.Join(res, ","))
}

type SecurityGroups struct {
	SecurityGroups []SecurityGroup `json:"SecurityGroups"`
}
type SecurityGroup struct {
	GroupId string `json:"GroupId"`
}

func (s SecurityGroup) String() string {
	return fmt.Sprintf(s.GroupId)
}

func (i SecurityGroups) String() string {
	var res []string
	for _, sg := range i.SecurityGroups {
		res = append(res, sg.String())
	}
	return strings.Join(res, ",")
}

type Attachment struct {
	State string `json:"State"`
	VpcId string `json:"VpcId"`
}

type InternetGateway struct {
	InternetGatewayId string       `json:"InternetGatewayId"`
	Attachments       []Attachment `json:"Attachments"`
}

type InternetGateways struct {
	InternetGateways []InternetGateway `json:"InternetGateways"`
}

type NewInternetGateway struct {
	InternetGateway InternetGateway `json:"InternetGateway"`
}

func (i InternetGateway) String() string {
	return fmt.Sprintf("InternetGatewayId: %s", i.InternetGatewayId)
}

func (i InternetGateways) String() string {
	var res []string
	for _, gw := range i.InternetGateways {
		res = append(res, gw.String())
	}
	return strings.Join(res, ",")
}

func (i NewInternetGateway) String() string {
	return i.InternetGateway.String()
}

type VPCStatus struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type VpcPeer struct {
	VpcPeeringConnectionId string    `json:"VpcPeeringConnectionId"`
	VpcStatus              VPCStatus `json:"Status"`
}

type VpcConnection struct {
	VpcPeeringConnection VpcPeer `json:"VpcPeeringConnection"`
}

type VpcConnections struct {
	VpcPeeringConnections []VpcPeer `json:"VpcPeeringConnections"`
}

type ResourceTag struct {
	clusterName    string
	clusterType    string
	subClusterType string
	port           []int
}

type CreateVpcPeering struct {
	user      string
	host      string
	sourceVPC ResourceTag
	targetVPC ResourceTag
}

func getVPCInfo(executor ctxt.Executor, ctx context.Context, vpc ResourceTag, vpcInfo *Vpc) error {
	fmt.Printf("Coming here to search for the vpc \n\n\n")
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\"", vpc.clusterName, vpc.clusterType, vpc.subClusterType), false)
	if err != nil {
		return err
	}

	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return err
	}
	if len(vpcs.Vpcs) > 1 {
		return errors.New("Multiple VPC found")
	}

	if len(vpcs.Vpcs) == 0 {
		return errors.New("No VPC found")
	}
	*vpcInfo = vpcs.Vpcs[0]

	return nil
}

func getVPC(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string) (*Vpc, error) {
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" ", clusterName, subClusterType, clusterType), false)
	if err != nil {
		return nil, err
	}
	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil, err
	}
	if len(vpcs.Vpcs) > 0 {

		return &vpcs.Vpcs[0], nil
		//c.clusterInfo.vpcInfo = vpcs.Vpcs[0]
		//zap.L().Info("The clusterInfo.vpcInfo.vpcId is ", zap.String("VpcInfo", c.clusterInfo.String()))
		//return nil
	}
	return nil, nil
}

func getNetworks(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType, scope string) (*[]Subnet, error) {
	command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=%s\"", clusterName, clusterType, subClusterType, scope)
	zap.L().Debug("Command", zap.String("describe-subnets", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil, err
	}

	var subnets Subnets
	if err = json.Unmarshal(stdout, &subnets); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil, err
	}
	return &subnets.Subnets, nil
}

func getNetworksString(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType, scope string) (string, error) {
	subnets, err := getNetworks(executor, ctx, clusterName, clusterType, subClusterType, scope)
	if err != nil {
		return "", err
	}
	if subnets == nil {
		return "", errors.New("No subnets found")
	}
	var arrSubnets []string
	for _, subnet := range *subnets {
		arrSubnets = append(arrSubnets, "\""+subnet.SubnetId+"\"")
	}
	return "[" + strings.Join(arrSubnets, ",") + "]", nil

}

func getTransitGateway(executor ctxt.Executor, ctx context.Context, clusterName string) (*TransitGateway, error) {
	command := fmt.Sprintf("aws ec2 describe-transit-gateways --filters \"Name=tag:Name,Values=%s\" \"Name=state,Values=available,modifying,pending\"", clusterName)
	stdout, stderr, err := executor.Execute(ctx, command, false)
	if err != nil {
		fmt.Printf("The error err here is <%#v> \n\n", err)
		fmt.Printf("----------\n\n")
		fmt.Printf("The error stderr here is <%s> \n\n", string(stderr))
		return nil, err
	} else {
		var transitGateways TransitGateways
		if err = json.Unmarshal(stdout, &transitGateways); err != nil {
			fmt.Printf("*** *** The error here is %#v \n\n", err)
			return nil, err
		}
		for _, transitGateway := range transitGateways.TransitGateways {
			return &transitGateway, nil
		}
	}
	return nil, nil
}
