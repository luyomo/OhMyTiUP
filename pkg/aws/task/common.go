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
	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/ctxt"
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
	"os"
	"path"
	"strings"
	"text/template"
	//	"github.com/luyomo/tisample/pkg/executor"
	//	"strings"
)

type Vpc struct {
	CidrBlock string `json:"CidrBlock"`
	State     string `json:"State"`
	VpcId     string `json:"VpcId"`
	OwnerId   string `json:"OwnerId"`
	Tags      []Tag  `json:"Tags"`
}

type Vpcs struct {
	Vpcs []Vpc `json:"Vpcs"`
}

type ClusterInfo struct {
	cidr                   string
	region                 string
	keyName                string
	keyFile                string
	instanceType           string
	imageId                string
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

func getVPCInfos(executor ctxt.Executor, ctx context.Context, vpc ResourceTag) (*Vpcs, error) {
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" ", vpc.clusterName, vpc.clusterType), false)
	if err != nil {
		return nil, err
	}

	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil, err
	}
	return &vpcs, nil
}

func getVPCInfo(executor ctxt.Executor, ctx context.Context, vpc ResourceTag) (*Vpc, error) {
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\"", vpc.clusterName, vpc.clusterType, vpc.subClusterType), false)
	if err != nil {
		return nil, err
	}

	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil, err
	}
	if len(vpcs.Vpcs) > 1 {
		return nil, errors.New("Multiple VPC found")
	}

	if len(vpcs.Vpcs) == 0 {
		return nil, errors.New("No VPC found")
	}
	return &(vpcs.Vpcs[0]), nil
}

func getNetworks(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType, scope string) (*[]Subnet, error) {
	//command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Scope\" \"Name=tag-value,Values=%s\"", clusterName, clusterType, subClusterType, scope)
	command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", clusterName, clusterType, subClusterType)
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

func getRouteTable(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string) (*RouteTable, error) {
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" ", clusterName, clusterType, subClusterType), false)
	if err != nil {
		return nil, err
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		zap.L().Error("Failed to parse the route table", zap.String("describe-route-table", string(stdout)))
		return nil, err
	}

	zap.L().Debug("Print the route tables", zap.String("routeTables", routeTables.String()))
	if len(routeTables.RouteTables) == 0 {
		return nil, errors.New("No route table found")
	}
	if len(routeTables.RouteTables) > 1 {
		return nil, errors.New("Multiple route tables found")
	}
	return &routeTables.RouteTables[0], nil
}

func getRouteTableByVPC(executor ctxt.Executor, ctx context.Context, clusterName, vpcID string) (*RouteTable, error) {
	stdout, _, err := executor.Execute(ctx, fmt.Sprintf("aws ec2 describe-route-tables --filters \"Name=tag:Name,Values=%s\" \"Name=vpc-id,Values=%s\" ", clusterName, vpcID), false)
	if err != nil {
		return nil, err
	}

	var routeTables RouteTables
	if err = json.Unmarshal(stdout, &routeTables); err != nil {
		zap.L().Error("Failed to parse the route table", zap.String("describe-route-table", string(stdout)))
		return nil, err
	}

	zap.L().Debug("Print the route tables", zap.String("routeTables", routeTables.String()))
	if len(routeTables.RouteTables) == 0 {
		return nil, errors.New("No route table found")
	}
	if len(routeTables.RouteTables) > 1 {
		return nil, errors.New("Multiple route tables found")
	}
	return &routeTables.RouteTables[0], nil
}

func getWorkstation(executor ctxt.Executor, ctx context.Context, clusterName, clusterType string) (*EC2, error) {
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=16\"", clusterName, clusterType, "workstation")
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil, err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil, err
	}

	var theInstance EC2
	cntInstance := 0
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			cntInstance++
			theInstance = instance
		}
	}

	if cntInstance > 1 {
		return nil, errors.New("Multiple workstation nodes")
	}
	if cntInstance == 0 {
		return nil, errors.New("No workstation node")
	}

	return &theInstance, nil
}

func getWSExecutor(texecutor ctxt.Executor, ctx context.Context, clusterName, clusterType, user, keyFile string) (*ctxt.Executor, error) {
	workstation, err := getWorkstation(texecutor, ctx, clusterName, clusterType)
	if err != nil {
		return nil, err
	}
	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: workstation.PublicIpAddress, User: user, KeyFile: keyFile})
	if err != nil {
		return nil, err
	}
	return &wsexecutor, nil
}

func getTiDBClusterInfo(wsexecutor *ctxt.Executor, ctx context.Context, clusterName, clusterType string) (*TiDBClusterDetail, error) {

	stdout, stderr, err := (*wsexecutor).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster display %s --format json `, clusterName), false)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
		return nil, err
	}

	var tidbClusterDetail TiDBClusterDetail
	if err = json.Unmarshal(stdout, &tidbClusterDetail); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
		return nil, err
	}

	return &tidbClusterDetail, nil
}

func getEC2Nodes(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, componentName string) (*[]EC2, error) {
	var reservations Reservations
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Component,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, componentName)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil, err
	}

	var theEC2s []EC2
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			theEC2s = append(theEC2s, instance)
		}
	}

	return &theEC2s, nil

}

// func deploy(executor ctxt.Executor, ctx context.Context, host string, port int) error {
// 	deployFreetds(executor, ctx, "REPLICA", host, port)
// 	return nil
// }

func deployFreetds(executor ctxt.Executor, ctx context.Context, name, host string, port int) error {

	_, _, err := executor.Execute(ctx, `apt-get install -y freetds-bin`, true)
	if err != nil {
		return nil
	}

	fdFile, err := os.Create(fmt.Sprintf("/tmp/%s", "freetds.conf"))
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp := path.Join("templates", "config", fmt.Sprintf("%s.tpl", "freetds.conf"))
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	var tplData TplSQLServer
	tplData.Name = name
	tplData.Host = host
	tplData.Port = port
	if err := tmpl.Execute(fdFile, tplData); err != nil {
		return err
	}

	err = executor.Transfer(ctx, fmt.Sprintf("/tmp/%s", "freetds.conf"), "/tmp/freetds.conf", false, 0)
	if err != nil {
		fmt.Printf("The error is <%#v> \n\n\n", err)
		return err
	}

	command := fmt.Sprintf(`mv /tmp/freetds.conf /etc/freetds/`)
	_, stderr, err := executor.Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
		return err
	}

	/*
		command = fmt.Sprintf(`bsqldb -i /opt/tidb/freetds.conf -S %s -U sa -P 1234@Abcd -i /opt/tidb/sql/ontime_ms.ddl`, tplData.Name)
		stdout, stderr, err = executor.Execute(ctx, command, false)
		if err != nil {
			fmt.Printf("The error here is <%#v> \n\n\n", string(stderr))
			return err
		}
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The result from command <%s> \n\n\n", string(stdout))
	*/
	return nil
}

/************************** The function for the [][]string sort **************/
type byComponentNameZone [][]string

func (items byComponentNameZone) Len() int      { return len(items) }
func (items byComponentNameZone) Swap(i, j int) { items[i], items[j] = items[j], items[i] }
func (items byComponentNameZone) Less(i, j int) bool {
	if items[i][0] < items[j][0] {
		return true
	}
	if items[i][0] == items[j][0] && items[i][1] < items[j][1] {
		return true
	}
	return false
}

type byComponentName [][]string

func (items byComponentName) Len() int      { return len(items) }
func (items byComponentName) Swap(i, j int) { items[i], items[j] = items[j], items[i] }
func (items byComponentName) Less(i, j int) bool {
	if items[i][0] < items[j][0] {
		return true
	}
	return false
}
