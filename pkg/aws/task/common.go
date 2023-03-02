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
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/aws/smithy-go"

	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/tidbcloudapi"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	// "github.com/aws/aws-sdk-go-v2/service/s3/types"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
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
	excludedAZ             []string
	includedAZ             []string
	enableNAT              string
}

type DBInfo struct {
	DBHost     string
	DBPort     int64
	DBUser     string
	DBPassword string
}

type DMClusterInfo struct {
	Name       string `json:"name"`
	User       string `json:"user"`
	DMVersion  string `json:"version"`
	Path       string `json:"path"`
	PrivateKey string `json:"private_key"`
}

type DMClustersInfo struct {
	Clusters []DMClusterInfo `json:"clusters"`
}

func (v Vpc) String() string {
	return fmt.Sprintf("Cidr: %s, State: %s, VpcId: %s, OwnerId: %s", v.CidrBlock, v.State, v.VpcId, v.OwnerId)
}

func (c ClusterInfo) String() string {
	return fmt.Sprintf("vpcInfo:[%s], privateRouteTableId:%s, publicRouteTableId:%s, privateSecurityGroupId:%s, publicSecurityGroupId:%s, privateSubnets:%s, publicSubnet:%s, pcxTidb2Aurora:%s", c.vpcInfo.String(), c.privateRouteTableId, c.publicRouteTableId, c.privateSecurityGroupId, c.publicSecurityGroupId, strings.Join(c.privateSubnets, ","), c.publicSubnet, c.pcxTidb2Aurora)
}

type Route struct {
	DestinationCidrBlock string `json:"DestinationCidrBlock"`
	TransitGatewayId     string `json:"TransitGatewayId"`
	GatewayId            string `json:"GatewayId"`
	Origin               string `json:"Origin"`
	State                string `json:"State"`
}

type RouteTable struct {
	RouteTableId string  `json:"RouteTableId"`
	Tags         []Tag   `json:"Tags"`
	Routes       []Route `json:"Routes"`
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

type IpRanges struct {
	CidrIp string `json:"CidrIp"`
}

type IpPermissions struct {
	FromPort   int        `json:"FromPort"`
	IpProtocol string     `json:"IpProtocol"`
	IpRanges   []IpRanges `json:"IpRanges"`
	ToPort     int        `json:"ToPort"`
}

type SecurityGroups struct {
	SecurityGroups []SecurityGroup `json:"SecurityGroups"`
}

type SecurityGroup struct {
	GroupId       string          `json:"GroupId"`
	GroupName     string          `json:"GroupName"`
	IpPermissions []IpPermissions `json:"IpPermissions"`
	Tags          []Tag           `json:"Tags"`
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

type DMTaskDetail struct {
	Result  bool   `json:"result"`
	Msg     string `json:"msg"`
	Sources []struct {
		Result       bool   `json:"result"`
		Msg          string `json:"msg"`
		SourceStatus struct {
			Source      string `json:"source"`
			Worker      string `json:"worker"`
			Result      string `json:"result"`
			RelayStatus string `json:"relayStatus"`
		} `json:"sourceStatus"`
		SubTaskStatus []struct {
			Name                string `json:"name"`
			Stage               string `json:"stage"`
			Unit                string `json:"unit"`
			Result              string `json:"result"`
			UnresolvedDDLLockID string `json:"unresolvedDDLLockID"`
			Sync                struct {
				TotalEvents         string   `json:"totalEvents"`
				TotalTps            string   `json:"totalTps"`
				RecentTps           string   `json:"recentTps"`
				MasterBinlog        string   `json:"masterBinlog"`
				MasterBinlogGtid    string   `json:"masterBinlogGtid"`
				SyncerBinlog        string   `json:"syncerBinlog"`
				SyncerBinlogGtid    string   `json:"syncerBinlogGtid"`
				BlockingDDLs        []string `json:"blockingDDLs"`
				UnresolvedGroups    []string `json:"unresolvedGroups"`
				Synced              bool     `json:"synced"`
				BinlogType          string   `json:"binlogType"`
				SecondsBehindMaster string   `json:"secondsBehindMaster"`
				BlockDDLOwner       string   `json:"blockDDLOwner"`
				ConflictMsg         string   `json:"conflictMsg"`
			} `json:"sync"`
		} `json:"subTaskStatus"`
	} `json:"sources"`
}

type DisplayDMCluster struct {
	ClusterMeta struct {
		ClusterType    string `json:"cluster_type"`
		ClusterName    string `json:"cluster_name"`
		ClusterVersion string `json:"cluster_version"`
		DeployUser     string `json:"deploy_user"`
		SshType        string `json:"ssh_type"`
		TlsEnabled     bool   `json:"tls_enabled"`
	} `json:"cluster_meta"`
	Instances []struct {
		ID            string `json:"id"`
		Role          string `json:"role"`
		Host          string `json:"host"`
		Ports         string `json:"ports"`
		OsArch        string `json:"os_arch"`
		Status        string `json:"status"`
		Since         string `json:"since"`
		DataDir       string `json:"data_dir"`
		DeployDir     string `json:"deploy_dir"`
		ComponentName string `json:"ComponentName"`
		Port          int    `json:"Port"`
	} `json:"instances"`
}

func contains(s *[]map[string]string, str string) bool {
	for _, v := range *s {
		if v["Cluster"] == str {
			return true
		}
	}

	return false
}

func SearchVPCName(executor *ctxt.Executor, ctx context.Context, clusterKeyWord string) (*[]map[string]string, error) {
	stdout, _, err := (*executor).Execute(ctx, fmt.Sprintf("aws ec2 describe-vpcs --filters Name=tag:Cluster,Values=%s ", clusterKeyWord), false)
	if err != nil {
		return nil, err
	}
	var retValue []map[string]string

	var vpcs Vpcs
	if err := json.Unmarshal(stdout, &vpcs); err != nil {
		zap.L().Debug("The error to parse the string ", zap.Error(err))
		return nil, err
	}
	for _, vpc := range vpcs.Vpcs {
		entry := make(map[string]string)
		for _, tag := range vpc.Tags {
			if tag.Key == "Name" {
				entry["Name"] = tag.Value
			}
			if tag.Key == "Type" {
				entry["Type"] = tag.Value
			}
		}
		if !contains(&retValue, entry["Cluster"]) {
			retValue = append(retValue, entry)
		}
	}

	return &retValue, nil

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
	command := fmt.Sprintf("aws ec2 describe-subnets --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\"", clusterName, clusterType, subClusterType)
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

type AptLock struct {
	Locks []struct {
		//"command":"cron", "pid":686, "type":"FLOCK", "size":null, "mode":"WRITE", "m":false, "start":0, "end":0, "path":"/run..."
		Command string `json:"command"`
		Pid     int32  `json:"pid"`
		Type    string `json:"type"`
		Size    string `json:"size"`
		Mode    string `json:"mode"`
		M       bool   `json:"m"`
		Start   int32  `json:"start"`
		End     int32  `json:"end"`
		Path    string `json:"path"`
	} `json:"locks"`
}

func LookupAptLock(appName string, inStr []byte) (bool, error) {
	var aptLock AptLock
	if err := json.Unmarshal(inStr, &aptLock); err != nil {
		return false, err
	}
	for _, _entry := range aptLock.Locks {
		if _entry.Command == appName {
			return true, nil
		}
	}

	return false, nil
}

func getTransitGateway(executor ctxt.Executor, ctx context.Context, clusterName, clusterType string) (*TransitGateway, error) {
	command := fmt.Sprintf("aws ec2 describe-transit-gateways --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=state,Values=available,modifying,pending\"", clusterName, clusterType)
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return nil, err
	} else {
		var transitGateways TransitGateways
		if err = json.Unmarshal(stdout, &transitGateways); err != nil {
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

func GetWSExecutor(texecutor ctxt.Executor, ctx context.Context, clusterName, clusterType, user, keyFile string) (*ctxt.Executor, error) {
	workstation, err := getWorkstation(texecutor, ctx, clusterName, clusterType)
	if err != nil {
		return nil, err
	}

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: workstation.PublicIpAddress, User: user, KeyFile: keyFile}, []string{})
	if err != nil {
		return nil, err
	}
	//lsb_release --id
	return &wsexecutor, nil
}

func GetWSExecutor02(texecutor ctxt.Executor, ctx context.Context, clusterName, clusterType, user, keyFile string, awsCliFlag bool, args *interface{}) (*ctxt.Executor, error) {
	var envs []string

	if awsCliFlag == true {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return nil, err
		}
		// fmt.Printf("Account id is: <%#v> \n\n\n", cfg)

		envs = append(envs, fmt.Sprintf("AWS_DEFAULT_REGION=%s", cfg.Region))

		crentials, err := cfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			return nil, err
		}

		envs = append(envs, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", crentials.AccessKeyID))
		envs = append(envs, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", crentials.SecretAccessKey))
	}

	workstation, err := getWorkstation(texecutor, ctx, clusterName, clusterType)
	if err != nil {
		return nil, err
	}

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: workstation.PublicIpAddress, User: user, KeyFile: keyFile}, envs)
	if err != nil {
		return nil, err
	}
	//lsb_release --id
	return &wsexecutor, nil
}

func containString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func containInt64(s []uint64, e uint64) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func installPKGs(wsexecutor *ctxt.Executor, ctx context.Context, packages []string) error {
	stdout, _, err := (*wsexecutor).Execute(ctx, "lsb_release --id", true)
	if err != nil {
		return err
	}
	osVersion := strings.Split(string(stdout), ":")

	for _, pkg := range packages {
		if containString([]string{"Debian"}, strings.TrimSpace(osVersion[1])) {
			if _, _, err := (*wsexecutor).Execute(ctx, fmt.Sprintf("apt-get install -y %s", pkg), true); err != nil {
				return err
			}
		} else {
			if _, _, err := (*wsexecutor).Execute(ctx, fmt.Sprintf("yum install -y %s", pkg), true); err != nil {
				return err
			}
		}

	}
	return nil

}

func getTiDBClusterInfo(wsexecutor *ctxt.Executor, ctx context.Context, clusterName string) (*TiDBClusterDetail, error) {

	stdout, _, err := (*wsexecutor).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup cluster display %s --format json `, clusterName), false)
	if err != nil {
		return nil, err
	}

	var tidbClusterDetail TiDBClusterDetail
	if err = json.Unmarshal(stdout, &tidbClusterDetail); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
		return nil, err
	}

	return &tidbClusterDetail, nil
}

func getDMClusterInfo(wsexecutor *ctxt.Executor, ctx context.Context, clusterName string) (*DMClusterInfo, error) {

	stdout, _, err := (*wsexecutor).Execute(ctx, fmt.Sprintf(`/home/admin/.tiup/bin/tiup dm list --format json`), false)
	if err != nil {
		return nil, err
	}

	var dmClustersInfo DMClustersInfo
	if err = json.Unmarshal(stdout, &dmClustersInfo); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("tidb cluster list", string(stdout)))
		return nil, err
	}

	for _, clusterInfo := range dmClustersInfo.Clusters {
		if clusterInfo.Name == clusterName {
			return &clusterInfo, nil
		}
	}

	return nil, nil
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

func deployFreetds(executor ctxt.Executor, ctx context.Context, name, host string, port int) error {

	if err := installPKGs(&executor, ctx, []string{"freetds-bin"}); err != nil {
		return err
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
		return err
	}

	command := fmt.Sprintf(`mv /tmp/freetds.conf /etc/freetds/`)
	_, _, err = executor.Execute(ctx, command, true)
	if err != nil {
		return err
	}

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

// type TargetGroups struct {
// 	TargetGroups []TargetGroup `json:"TargetGroups"`
// }

// type TargetGroup struct {
// 	TargetGroupArn  string `json:"TargetGroupArn"`
// 	TargetGroupName string `json:"TargetGroupName"`
// 	Protocol        string `json:"Protocol"`
// 	Port            int    `json:"Port"`
// 	VpcId           string `json:"VpcId"`
// 	TargetType      string `json:"TargetType"`
// }

type TagDescription struct {
	Tags []Tag `json:"Tags"`
}

type TagDescriptions struct {
	TagDescriptions []TagDescription `json:"TagDescriptions"`
}

func ExistsELBResource(executor ctxt.Executor, ctx context.Context, clusterType, subClusterType, clusterName, resourceName string) bool {
	command := fmt.Sprintf("aws elbv2 describe-tags --resource-arns %s ", resourceName)
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return false
	}

	var tagDescriptions TagDescriptions
	if err = json.Unmarshal(stdout, &tagDescriptions); err != nil {
		return false
	}
	matchedCnt := 0
	for _, tagDescription := range tagDescriptions.TagDescriptions {
		for _, tag := range tagDescription.Tags {
			if tag.Key == "Cluster" && tag.Value == clusterType {
				matchedCnt++
			}
			if tag.Key == "Type" && tag.Value == subClusterType {
				matchedCnt++
			}
			if tag.Key == "Name" && tag.Value == clusterName {
				matchedCnt++
			}
			if matchedCnt == 3 {
				return true
			}
		}
	}
	return false
}

func getTargetGroup(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string) (*elbtypes.TargetGroup, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	clientElb := elb.NewFromConfig(cfg)
	describeTargetGroups, err := clientElb.DescribeTargetGroups(context.TODO(), &elb.DescribeTargetGroupsInput{Names: []string{clusterName}})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "TargetGroupNotFound" {
				return nil, nil
			}
		}

		return nil, err
	}
	return &describeTargetGroups.TargetGroups[0], nil

	// command := fmt.Sprintf("aws elbv2 describe-target-groups --name \"%s\"", clusterName)
	// stdout, stderr, err := executor.Execute(ctx, command, false)
	// if err != nil {
	// 	if strings.Contains(string(stderr), "One or more target groups not found") {
	// 		return nil, errors.New("No target group found")
	// 	} else {
	// 		return nil, err
	// 	}
	// }
	// var targetGroups TargetGroups
	// if err = json.Unmarshal(stdout, &targetGroups); err != nil {
	// 	return nil, err
	// }

	// for _, targetGroup := range targetGroups.TargetGroups {
	// 	if existsResource := ExistsELBResource(executor, ctx, clusterType, subClusterType, clusterName, targetGroup.TargetGroupArn); existsResource == true {
	// 		return &targetGroup, nil
	// 	}
	// }
	// return nil, errors.New("No target group found")
}

// func getNLB(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string) (*LoadBalancer, error) {
func getNLB(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string) (*elbtypes.LoadBalancer, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	clientElb := elb.NewFromConfig(cfg)

	describeLoadBalancers, err := clientElb.DescribeLoadBalancers(context.TODO(), &elb.DescribeLoadBalancersInput{Names: []string{clusterName}})
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

	return &describeLoadBalancers.LoadBalancers[0], nil

	// command := fmt.Sprintf("aws elbv2 describe-load-balancers --name \"%s\"", clusterName)
	// stdout, stderr, err := executor.Execute(ctx, command, false)
	// if err != nil {
	// 	// var ae smithy.APIError
	// 	// if errors.As(err, &ae) {
	// 	// 	fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
	// 	// 	if ae.ErrorCode() == "LoadBalancerNotFound" {
	// 	// 		return nil, nil
	// 	// 	}
	// 	// }

	// 	// return nil, err

	// 	if strings.Contains(string(stderr), fmt.Sprintf("Load balancers '[%s]' not found", clusterName)) {
	// 		return nil, errors.New("No NLB found")
	// 	} else {
	// 		return nil, err
	// 	}
	// }
	// var loadBalancers LoadBalancers
	// if err = json.Unmarshal(stdout, &loadBalancers); err != nil {
	// 	return nil, err
	// }

	// for _, loadBalancer := range loadBalancers.LoadBalancers {
	// 	if existsResource := ExistsELBResource(executor, ctx, clusterType, subClusterType, clusterName, loadBalancer.LoadBalancerArn); existsResource == true {
	// 		return &loadBalancer, nil
	// 	}
	// }
	// return nil, errors.New("No NLB found")
}

func installWebSSH2(wexecutor *ctxt.Executor, ctx context.Context) error {

	if err := installPKGs(wexecutor, ctx, []string{"nodejs", "npm", "cmake"}); err != nil {
		return err
	}

	commands := []string{"[ -d /opt/webssh2 ] || git clone https://github.com/billchurch/webssh2.git /opt/webssh2", "npm install /opt/webssh2/app"}

	for _, command := range commands {
		_, _, err := (*wexecutor).Execute(ctx, command, true)
		if err != nil {
			return err
		}
	}

	err := (*wexecutor).Transfer(ctx, "embed/templates/systemd/webssh2.service", "/tmp/", false, 0)
	if err != nil {
		return err
	}

	_, _, err = (*wexecutor).Execute(ctx, "mv /tmp/webssh2.service /etc/systemd/system/", true)
	if err != nil {
		return err
	}

	return nil
}

func containsInArray(s []string, searchterm string) bool {

	if len(s) == 0 {
		return false
	}
	i := sort.SearchStrings(s, searchterm)

	return i < len(s) && s[i] == searchterm
}

type DBConnectInfo struct {
	DBHost     string `yaml:"Host"`
	DBPort     int    `yaml:"Port"`
	DBUser     string `yaml:"User"`
	DBPassword string `yaml:"Password"`
}

func ReadTiDBConntionInfo(workstation *ctxt.Executor, fileName string) (*DBConnectInfo, error) {

	// 02. Get the TiDB connection info
	// if err := (*workstation).Transfer(context.Background(), fmt.Sprintf("/opt/tidb-db-info.yml"), "/tmp/tidb-db-info.yml", true, 1024); err != nil {
	if err := (*workstation).Transfer(context.Background(), fmt.Sprintf("/opt/%s", fileName), fmt.Sprintf("/tmp/%s", fileName), true, 1024); err != nil {
		return nil, err
	}

	dbConnectInfo := DBConnectInfo{}

	yfile, err := ioutil.ReadFile(fmt.Sprintf("/tmp/%s", fileName))
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(yfile, &dbConnectInfo); err != nil {
		return nil, err
	}

	return &dbConnectInfo, nil
}

func ReadDBConntionInfo(workstation *ctxt.Executor, fileName string, connInfo interface{}) error {
	if err := (*workstation).Transfer(context.Background(), fmt.Sprintf("/opt/%s", fileName), fmt.Sprintf("/tmp/%s", fileName), true, 1024); err != nil {
		return err
	}

	yfile, err := ioutil.ReadFile(fmt.Sprintf("/tmp/%s", fileName))
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(yfile, connInfo); err != nil {
		return err
	}

	return nil
}

func TransferToWorkstation(workstation *ctxt.Executor, sourceFile, destFile, mode string, params interface{}) error {

	ctx := context.Background()

	err := (*workstation).TransferTemplate(ctx, sourceFile, fmt.Sprintf("/tmp/%s", "test.file"), mode, params, true, 0)
	if err != nil {
		return err
	}

	if _, _, err := (*workstation).Execute(ctx, fmt.Sprintf("mv /tmp/%s %s", "test.file", destFile), true); err != nil {
		return err
	}

	return nil

}

func ParseRangeData(inputData string) (*[]int, error) {

	var varRet []int

	numberPattern := regexp.MustCompile(`^\d+$`)
	rangePattern := regexp.MustCompile(`^(\d+)-(\d+)/(\d+)$`)

	match := numberPattern.MatchString(inputData)
	if match == false {
		dataRange := rangePattern.FindStringSubmatch(inputData)

		if dataRange == nil {
			return nil, errors.New("Not match user num pattern")
		}
		num, _ := strconv.Atoi(dataRange[1])
		endNum, _ := strconv.Atoi(dataRange[2])
		interval, _ := strconv.Atoi(dataRange[3])

		for {
			if num > endNum {
				break
			}

			varRet = append(varRet, num)
			num += interval
		}

	} else {
		intNum, err := strconv.Atoi(inputData)
		if err != nil {
			return nil, err
		}
		varRet = append(varRet, intNum)
	}

	return &varRet, nil
}

func ConvertEpochToString(strTimestamp string) string {

	i, err := strconv.ParseInt(strTimestamp, 10, 64)
	if err != nil {
		panic(err)
	}

	t := time.Unix(i, 0)

	strDate := t.Format("2006-01-02 15:04:05")
	return strDate

}

func InitClientInstance() error {
	// Get User/Password from variables
	var (
		publicKey  = os.Getenv("TIDBCLOUD_PUBLIC_KEY")
		privateKey = os.Getenv("TIDBCLOUD_PRIVATE_KEY")
	)
	if publicKey == "" || privateKey == "" {
		fmt.Printf("Please set TIDBCLOUD_PUBLIC_KEY(%s), TIDBCLOUD_PRIVATE_KEY(%s) in environment variable first\n", publicKey, privateKey)
		return errors.New("Missed public/private key")
	}

	// Client initialization
	err := tidbcloudapi.InitClient(publicKey, privateKey)
	if err != nil {
		fmt.Printf("Failed to init HTTP client\n")
		return err
	}

	return nil
}

// Get the user from increntials for resource tag addition
func GetCallerUser(ctx context.Context) (string, string, error) {
	if ctx.Value("tagOwner") != nil && ctx.Value("tagOwner") != "" {
		return ctx.Value("tagOwner").(string), "", nil
	}

	_ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(_ctx)
	if err != nil {
		return "", "", err
	}

	_client := sts.NewFromConfig(cfg)

	var _getCallerIdentityInput *sts.GetCallerIdentityInput

	_caller, err := _client.GetCallerIdentity(ctx, _getCallerIdentityInput)

	return strings.Split((*_caller.Arn), "/")[1], *_caller.Account, nil
}

// func GetAWSCrential(ctx context.Context) (*aws.Credentials, error) {
func GetAWSCrential() (*aws.Credentials, error) {
	_ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(_ctx)
	if err != nil {
		return nil, err
	}

	_crentials, err := cfg.Credentials.Retrieve(_ctx)
	return &_crentials, nil
}

func GetS3BucketLocation(ctx context.Context, bucketName string) error {
	_ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(_ctx)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg)

	getBucketLocationInput := &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	}

	getBucketLocationOutput, err := client.GetBucketLocation(context.TODO(), getBucketLocationInput)
	if err != nil {
		return err
	}
	fmt.Printf("The bucket information is <%#v> \n\n\n\n", getBucketLocationOutput)

	return nil
}

// Get the project name. If it is not specified, user the cluster name as the project name
func GetProject(ctx context.Context) string {
	if ctx.Value("tagProject") != nil && ctx.Value("tagProject") != "" {
		return ctx.Value("tagProject").(string)
	}
	return ctx.Value("clusterName").(string)
}

/* ********** ********** ********** Generate node group for k8s ********* ********** ********** */
type DeployEKSNodeGroup struct {
	pexecutor         *ctxt.Executor
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
	nodeGroupName     string
}

// Execute implements the Task interface
func (c *DeployEKSNodeGroup) Execute(ctx context.Context) error {
	// 01. Get the context variables
	clusterName := ctx.Value("clusterName").(string)
	// clusterType := ctx.Value("clusterType").(string)

	// tagProject := GetProject(ctx)
	// tagOwner, _, err := GetCallerUser(ctx)
	// if err != nil {
	// 	return err
	// }

	// 02. Get context config
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	// 03. Get the role arn
	clientIam := iam.NewFromConfig(cfg)

	var roleArn string
	getRoleInput := &iam.GetRoleInput{RoleName: aws.String(clusterName)}

	getRoleOutput, err := clientIam.GetRole(context.TODO(), getRoleInput)
	if err != nil {
		return err
	}
	roleArn = *getRoleOutput.Role.Arn

	// 05. Create the node group if it does not exist
	clientEks := eks.NewFromConfig(cfg)

	listNodegroupsInput := &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)}
	listNodegroup, err := clientEks.ListNodegroups(context.TODO(), listNodegroupsInput)
	if err != nil {
		return err
	}

	if containString(listNodegroup.Nodegroups, c.nodeGroupName) == false {

		// Node group creation
		nodegroupScalingConfig := &types.NodegroupScalingConfig{DesiredSize: aws.Int32(1), MaxSize: aws.Int32(1), MinSize: aws.Int32(1)}
		createNodegroupInput := &eks.CreateNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodeRole:      aws.String(roleArn),
			NodegroupName: aws.String(c.nodeGroupName),
			Subnets:       c.clusterInfo.privateSubnets,
			InstanceTypes: []string{"c5.xlarge"},
			DiskSize:      aws.Int32(20),
			ScalingConfig: nodegroupScalingConfig}

		_, err := clientEks.CreateNodegroup(context.TODO(), createNodegroupInput)
		if err != nil {
			return err
		}

		// CREATING -> ACTIVE
		for _idx := 0; _idx < 100; _idx++ {

			describeNodegroupInput := &eks.DescribeNodegroupInput{ClusterName: aws.String(clusterName), NodegroupName: aws.String(c.nodeGroupName)}
			describeNodegroup, err := clientEks.DescribeNodegroup(context.TODO(), describeNodegroupInput)
			if err != nil {
				return err
			}

			if describeNodegroup.Nodegroup.Status == "ACTIVE" {
				break
			}
			if describeNodegroup.Nodegroup.Status == "CREATE_FAILED" {
				return errors.New(fmt.Sprintf("Failed to create node group %s", c.nodeGroupName))
			}

			time.Sleep(30 * time.Second)
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployEKSNodeGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployEKSNodeGroup) String() string {
	return fmt.Sprintf("Echo: Deploying EKS Cluster")
}

type DestroyEKSNodeGroup struct {
	pexecutor         *ctxt.Executor
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
	nodeGroupName     string
}

// Execute implements the Task interface
func (c *DestroyEKSNodeGroup) Execute(ctx context.Context) error {
	// 01. Get the context variables
	clusterName := ctx.Value("clusterName").(string)
	// clusterType := ctx.Value("clusterType").(string)

	// tagProject := GetProject(ctx)
	// tagOwner, _, err := GetCallerUser(ctx)
	// if err != nil {
	// 	return err
	// }

	// 02. Get context config
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	// 05. Create the node group if it does not exist
	clientEks := eks.NewFromConfig(cfg)

	listNodegroupsInput := &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)}
	listNodegroup, err := clientEks.ListNodegroups(context.TODO(), listNodegroupsInput)
	if err != nil {
		return err
	}

	if containString(listNodegroup.Nodegroups, c.nodeGroupName) == true {

		// Node group creation
		deleteNodegroupInput := &eks.DeleteNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(c.nodeGroupName),
		}

		_, err := clientEks.DeleteNodegroup(context.TODO(), deleteNodegroupInput)
		if err != nil {
			return err
		}

		// CREATING -> ACTIVE
		for _idx := 0; _idx < 100; _idx++ {

			listNodegroup, err := clientEks.ListNodegroups(context.TODO(), listNodegroupsInput)
			if err != nil {
				return err
			}

			if containString(listNodegroup.Nodegroups, c.nodeGroupName) == false {
				return nil
			}

			time.Sleep(30 * time.Second)
		}
		return errors.New("Failed to delete the node group")
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyEKSNodeGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyEKSNodeGroup) String() string {
	return fmt.Sprintf("Echo: Deploying EKS Cluster")
}

// [{"name":"nginx-ingress-controller","namespace":"default","revision":"1","updated":"2023-01-05 14:48:14.781865271 +0000 UTC","status":"deployed","chart":"ingress-nginx-4.4.2","app_version":"1.5.1"}]
type HelmListInfo struct {
	Name       string `json:"name"`
	NameSpace  string `json:"namespace"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:status`
	Chart      string `json:chart`
	AppVersion string `json:"app_version"`
}

func HelmResourceExist(executor *ctxt.Executor, resourceName string) (bool, error) {
	var helmListInfos []HelmListInfo

	stdout, _, err := (*executor).Execute(context.TODO(), `helm list -o json`, false)
	if err != nil {
		return false, err
	}

	if err = json.Unmarshal(stdout, &helmListInfos); err != nil {
		return false, err
	}

	for _, helmListInfo := range helmListInfos {
		if helmListInfo.Name == resourceName {
			return true, nil
		}
	}
	return false, nil
}

type IAMSAInfo struct {
	MetaData struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	WellKnownPolicies struct {
		ImageBuilder              bool `json:"imageBuilder"`
		AutoScaler                bool `json:"autoScaler"`
		AwsLoadBalancerController bool `json:"awsLoadBalancerController"`
		ExternalDNS               bool `json:"externalDNS"`
		CertManager               bool `json:"certManager"`
		EbsCSIController          bool `json:"ebsCSIController"`
		EfsCSIController          bool `json:"efsCSIController"`
	} `json:"wellKnownPolicies"`
	Status struct {
		RoleARN string `json:"roleARN"`
	} `json:"status"`
}

func FetchClusterSA(executor *ctxt.Executor, clusterName string) (*[]IAMSAInfo, error) {
	var iamSAInfos []IAMSAInfo

	stdout, _, err := (*executor).Execute(context.TODO(), fmt.Sprintf(`eksctl get iamserviceaccount --cluster %s --namespace kube-system -o json`, clusterName), false)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(stdout, &iamSAInfos); err != nil {
		return nil, err
	}

	return &iamSAInfos, nil
}

func CleanClusterSA(executor *ctxt.Executor, clusterName string) error {
	clusterSA, err := FetchClusterSA(executor, clusterName)
	if err != nil {
		return err
	}

	for _, sa := range *clusterSA {

		cmd := fmt.Sprintf(`eksctl delete iamserviceaccount %s --namespace kube-system --cluster %s`, sa.MetaData.Name, clusterName)
		fmt.Printf("----- ----- ----- Deleting iamserviceaccount: <%s> \n\n\n\n", cmd)
		if _, _, err := (*executor).Execute(context.TODO(), cmd, false); err != nil {
			return err
		}
	}
	return nil
}
