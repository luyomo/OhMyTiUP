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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	yaml "gopkg.in/yaml.v3"

	// "reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	smithy "github.com/aws/smithy-go"

	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
	"github.com/luyomo/OhMyTiUP/pkg/tidbcloudapi"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	// "github.com/aws/aws-sdk-go-v2/service/s3/types"
	astypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	nlbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"

	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
)

type ReadResourceMode int

const (
	ReadResourceModeCommon        ReadResourceMode = 0
	ReadResourceModeBeforeCreate  ReadResourceMode = 1
	ReadResourceModeAfterCreate   ReadResourceMode = 2
	ReadResourceModeBeforeDestroy ReadResourceMode = 3
	ReadResourceModeAfterDestroy  ReadResourceMode = 4
)

type NetworkType string

const (
	NetworkTypeNAT     NetworkType = "nat"
	NetworkTypePublic  NetworkType = "public"
	NetworkTypePrivate NetworkType = "private"
)

type ThrowErrorFlag bool

const (
	ThrowErrorIfNotExists ThrowErrorFlag = true
	ContinueIfNotExists   ThrowErrorFlag = false
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
	subnetsNum             int
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

func getNextCidr(cidr string, idx int) string {
	ip := strings.Split(cidr, "/")[0]
	ipSegs := strings.Split(ip, ".")

	return ipSegs[0] + "." + ipSegs[1] + "." + strconv.Itoa(idx) + ".0/24"
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
		return nil, errors.New("getVPCInfo: No VPC found")
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
		return nil, errors.New("Multiple route tables found 02")
	}
	return &routeTables.RouteTables[0], nil
}

func GetWorkstation(executor ctxt.Executor, ctx context.Context) (*EC2, error) {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

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
	workstation, err := GetWorkstation(texecutor, ctx)
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
		envs = append(envs, fmt.Sprintf("AWS_SESSION_TOKEN=%s", crentials.SessionToken))
	}

	workstation, err := GetWorkstation(texecutor, ctx)
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

func GetWSExecutor03(texecutor ctxt.Executor, ctx context.Context, clusterName, clusterType, user, keyFile string, awsCliFlag bool, args *interface{}) (ctxt.Executor, error) {
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
		envs = append(envs, fmt.Sprintf("AWS_SESSION_TOKEN=%s", crentials.SessionToken))
	}

	workstation, err := GetWorkstation(texecutor, ctx)
	if err != nil {
		return nil, err
	}

	wsexecutor, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: workstation.PublicIpAddress, User: user, KeyFile: keyFile}, envs)
	if err != nil {
		return nil, err
	}
	//lsb_release --id
	return wsexecutor, nil
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

func getEC2Nodes(ctx context.Context, executor ctxt.Executor, clusterName, clusterType, componentName string) (*[]EC2, error) {
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

func deployFreetds(ctx context.Context, executor ctxt.Executor, name, host string, port int) error {

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

type TagDescription struct {
	Tags []Tag `json:"Tags"`
}

type TagDescriptions struct {
	TagDescriptions []TagDescription `json:"TagDescriptions"`
}

func ExistsELBResource(ctx context.Context, executor ctxt.Executor, clusterType, subClusterType, clusterName, resourceName string) bool {
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

func getTargetGroup(ctx context.Context, executor ctxt.Executor, clusterName, clusterType, subClusterType string) (*nlbtypes.TargetGroup, error) {
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
}

// func getNLB(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string) (*LoadBalancer, error) {
func getNLB(ctx context.Context, executor ctxt.Executor, clusterName, clusterType, subClusterType string) (*nlbtypes.LoadBalancer, error) {
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
}

func installWebSSH2(ctx context.Context, wexecutor *ctxt.Executor) error {

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

func ListContainElement(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

type DBConnectInfo struct {
	DBHost     string `yaml:"Host"`
	DBPort     int    `yaml:"Port"`
	DBUser     string `yaml:"User"`
	DBPassword string `yaml:"Password"`
}

func ReadDBConntionInfo(workstation *ctxt.Executor, fileName string, connInfo interface{}) error {
	if err := (*workstation).Transfer(context.Background(), fmt.Sprintf("/opt/%s", fileName), fmt.Sprintf("/tmp/%s", fileName), true, 1024); err != nil {
		return err
	}

	yfile, err := ioutil.ReadFile(fmt.Sprintf("/tmp/%s", fileName))
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yfile, connInfo)

	return err
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
func GetCallerUser() (string, string, error) {

	_ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(_ctx)
	if err != nil {
		return "", "", err
	}

	_client := sts.NewFromConfig(cfg)

	var _getCallerIdentityInput *sts.GetCallerIdentityInput

	_caller, err := _client.GetCallerIdentity(_ctx, _getCallerIdentityInput)

	callerInfo := strings.Split((*_caller.Arn), "/")
	if len(callerInfo) == 2 {
		return callerInfo[1], *_caller.Account, nil
	} else if len(callerInfo) == 3 {
		return callerInfo[2], *_caller.Account, nil
	}

	return "", *_caller.Account, nil

	// return strings.Split((*_caller.Arn), "/")[1], *_caller.Account, nil
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

type ResourceData interface {
	Reset() error
	Append(interface{})
	ResourceExist() (bool, error)
	ToPrintTable() *[][]string
	GetResourceArn(ThrowErrorFlag) (*string, error)
	GetData() []interface{}

	WriteIntoConfigFile(_fileName string) error
}

type BaseResourceInfo struct {
	Data []interface{}
}

func (b *BaseResourceInfo) WriteIntoConfigFile(_fileName string) error {
	marshalData, err := yaml.Marshal(b.Data)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(_fileName, marshalData, 0644)

	if err != nil {
		return err
	}
	return nil
}

func (d *BaseResourceInfo) Reset() error {
	d.Data = d.Data[:0]
	return nil
}

func (d *BaseResourceInfo) Append(data interface{}) {
	d.Data = append(d.Data, data)
}

func (d *BaseResourceInfo) GetData() []interface{} {
	return d.Data
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseResourceInfo) ResourceExist() (bool, error) {
	if len(b.Data) == 0 {
		return false, nil
	}
	if len(b.Data) > 1 {
		debug.PrintStack()
		return false, errors.New(fmt.Sprintf("Multiple resources found: <%#v>", b.Data))
	}
	return true, nil
}

func (b *BaseResourceInfo) GetResourceArn(throwErr ThrowErrorFlag, returnValue func(interface{}) (*string, error)) (*string, error) {
	if len(b.Data) == 0 {
		if throwErr == ThrowErrorIfNotExists {
			debug.PrintStack()
			return nil, errors.New("No resource found")
		}
		return nil, nil
	}
	if len(b.Data) > 1 {
		debug.PrintStack()
		return nil, errors.New(fmt.Sprintf("Multiple resources found: <%#v>", b.Data))
	}

	return returnValue(b.Data[0])
}

type TiDBInstanceInfo struct {
	ID            string `json:"id"`
	Role          string `json:"role"`
	Host          string `json:"host"`
	Ports         string `json:"ports"`
	OsArch        string `json:"os_arch"`
	Status        string `json:"status"`
	Memory        string `json:"memory"`
	MemoryLimit   string `json:"memory_limit"`
	CPUQuota      string `json:"cpu_quota"`
	Since         string `json:"since"`
	DataDir       string `json:"data_dir"`
	DeployDir     string `json:"deploy_dir"`
	ComponentName string `json:"ComponentName"`
	Port          int    `json:"Port"`
}

type TiDBClusterDisplay struct {
	ClusterMeta struct {
		ClusterType    string `json:"cluster_type"`
		ClusterName    string `json:"cluster_name"`
		ClusterVersion string `json:"cluster_version"`
		DeployUser     string `json:"deploy_user"`
		SshType        string `json:"ssh_type"`
		TlsEnabled     bool   `json:"tls_enabled"`
		DashboardUrl   string `json:"dashboard_url"`
	} `json:"cluster_meta"`
	Instances []TiDBInstanceInfo `json:"instances"`
}

type BaseTaskInterface interface {
	readResources(mode ReadResourceMode) error
}

type BaseWSTask struct {
	barMessage  string
	workstation *ws.Workstation

	innerTimer *awsutils.ExecutionTimer
	timer      *awsutils.ExecutionTimer
}

func (c *BaseWSTask) Rollback(ctx context.Context) error {
	return nil
}

func (c *BaseWSTask) String() string {
	return c.barMessage
}

func (c *BaseWSTask) startTimer() {
	c.innerTimer = awsutils.NewTimer()
}

func (c *BaseWSTask) takeInnerTimer(proc string) {
	if c.innerTimer != nil {
		c.innerTimer.Take(fmt.Sprintf("    -> %s", proc))
	}
}

func (c *BaseWSTask) completeTimer(jobName string) {
	c.takeInnerTimer(jobName)

	c.timer.Append(c.innerTimer)
}

func (c *BaseWSTask) takeTimer(jobName string) {
	if c.timer != nil {
		c.timer.Take(jobName)
	}
}

type BaseTask struct {
	BaseTaskInterface

	pexecutor *ctxt.Executor
	wsExe     *ctxt.Executor

	ResourceData ResourceData

	timer *awsutils.ExecutionTimer

	clusterName    string      // It's initialized from init() function
	clusterType    string      // It's initialized from init() function
	subClusterType string      // tidb/msk/workstation
	scope          NetworkType // public/private
	component      string      // tidb/tikv/pd

	clusterInfo *ClusterInfo
}

func (b *BaseTask) takeTimer(jobName string) {
	if b.timer != nil {
		b.timer.Take(jobName)
	}

}

func (b *BaseTask) getTiDBClusterInfo() (*TiDBClusterDisplay, error) {
	stdout, _, err := (*b.wsExe).Execute(context.Background(), fmt.Sprintf("~/.tiup/bin/tiup cluster display --format json %s", b.clusterName), false)
	if err != nil {
		return nil, err
	}

	var tidbClusterDisplay TiDBClusterDisplay
	if err = json.Unmarshal(stdout, &tidbClusterDisplay); err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to unmarshal data: <%s>, cluster name: <%s>", string(stdout), b.clusterName))
	}

	return &tidbClusterDisplay, nil
}

func (b *BaseTask) MakeEC2Tags() *[]ec2types.Tag {
	var tags []ec2types.Tag
	tags = append(tags, ec2types.Tag{Key: aws.String("Name"), Value: aws.String(b.clusterName)})
	tags = append(tags, ec2types.Tag{Key: aws.String("Cluster"), Value: aws.String(b.clusterType)})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		tags = append(tags, ec2types.Tag{Key: aws.String("Type"), Value: aws.String(b.subClusterType)})
	}

	if b.scope != "" {
		tags = append(tags, ec2types.Tag{Key: aws.String("Scope"), Value: aws.String(string(b.scope))})
	}

	if b.component != "" {
		tags = append(tags, ec2types.Tag{Key: aws.String("Component"), Value: aws.String(b.component)})
	}
	tags = append(tags, ec2types.Tag{Key: aws.String("Project"), Value: aws.String(b.clusterName)})

	owner, _, err := GetCallerUser()
	if err != nil {
		fmt.Printf("Failed to fetch the caller \n\n\n")
	}
	tags = append(tags, ec2types.Tag{Key: aws.String("Owner"), Value: aws.String(owner)})

	return &tags
}

func (b *BaseTask) MakeASTags() *[]astypes.Tag {
	var tags []astypes.Tag
	tags = append(tags, astypes.Tag{Key: aws.String("Name"), Value: aws.String(b.clusterName)})
	tags = append(tags, astypes.Tag{Key: aws.String("Cluster"), Value: aws.String(b.clusterType)})
	tags = append(tags, astypes.Tag{Key: aws.String("Project"), Value: aws.String(b.clusterName)})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		tags = append(tags, astypes.Tag{Key: aws.String("Type"), Value: aws.String(b.subClusterType)})
	}

	if b.scope != "" {
		tags = append(tags, astypes.Tag{Key: aws.String("Scope"), Value: aws.String(string(b.scope))})
	}

	if b.component != "" {
		tags = append(tags, astypes.Tag{Key: aws.String("Component"), Value: aws.String(b.component)})
	}
	owner, _, err := GetCallerUser()
	if err != nil {
		fmt.Printf("Failed to fetch the caller \n\n\n")
	}
	tags = append(tags, astypes.Tag{Key: aws.String("Owner"), Value: aws.String(owner)})

	return &tags
}

func (b *BaseTask) MakeNLBTags() *[]nlbtypes.Tag {
	var tags []nlbtypes.Tag
	tags = append(tags, nlbtypes.Tag{Key: aws.String("Name"), Value: aws.String(b.clusterName)})
	tags = append(tags, nlbtypes.Tag{Key: aws.String("Cluster"), Value: aws.String(b.clusterType)})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		tags = append(tags, nlbtypes.Tag{Key: aws.String("Type"), Value: aws.String(b.subClusterType)})
	}

	if b.scope != "" {
		tags = append(tags, nlbtypes.Tag{Key: aws.String("Scope"), Value: aws.String(string(b.scope))})
	}

	return &tags
}

func (b *BaseTask) MakeEC2Filters() *[]ec2types.Filter {
	var filters []ec2types.Filter

	filters = append(filters, ec2types.Filter{Name: aws.String("tag:Name"), Values: []string{b.clusterName}})
	filters = append(filters, ec2types.Filter{Name: aws.String("tag:Cluster"), Values: []string{b.clusterType}})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("tag:Type"), Values: []string{b.subClusterType}})
	}

	if b.scope != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("tag:Scope"), Values: []string{string(b.scope)}})
	}

	if b.component != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("tag:Component"), Values: []string{b.component}})
	}

	return &filters
}

func (b *BaseTask) MakeASFilters() *[]astypes.Filter {
	var filters []astypes.Filter

	filters = append(filters, astypes.Filter{Name: aws.String("tag:Name"), Values: []string{b.clusterName}})
	filters = append(filters, astypes.Filter{Name: aws.String("tag:Cluster"), Values: []string{b.clusterType}})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		filters = append(filters, astypes.Filter{Name: aws.String("tag:Type"), Values: []string{b.subClusterType}})
	}

	if b.scope != "" {
		filters = append(filters, astypes.Filter{Name: aws.String("tag:Scope"), Values: []string{string(b.scope)}})
	}

	if b.component != "" {
		filters = append(filters, astypes.Filter{Name: aws.String("tag:Component"), Values: []string{b.component}})
	}

	return &filters
}

func (b *BaseTask) GetSubnetsInfo(numSubnets int) (*[]string, error) {
	// Get the subnet for workstation
	// fmt.Printf("Info: name:%s, type: %s, clusterType:%s, scope: %s \n\n\n\n\n", b.clusterName, b.clusterType, b.subClusterType, b.scope)
	listSubnets := &ListSubnets{BaseSubnets: BaseSubnets{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope,
	}}}
	if err := listSubnets.Execute(nil); err != nil {
		return nil, err
	}

	return listSubnets.GetSubnets(numSubnets)
}

// If the subnet is in the assoicate, return nothing
// If there is no route table, return error
// If there is route table, the subnet is not in the association. return the table id.
func (b *BaseTask) GetRouteTable() (*ec2types.RouteTable, error) {
	// Get the subnet for workstation

	listRouteTable := &ListRouteTable{BaseRouteTable: BaseRouteTable{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope,
	}}}
	if err := listRouteTable.Execute(nil); err != nil {
		return nil, err
	}

	_data := listRouteTable.ResourceData.GetData()
	if len(_data) == 0 {
		return nil, errors.New("No route table found")
	}
	if len(_data) > 1 {
		fmt.Printf("Info: name:%s, type: %s, clusterType:%s, scope: %s \n\n\n\n\n", b.clusterName, b.clusterType, b.subClusterType, b.scope)
		debug.PrintStack()
		return nil, errors.New("Multiple route tables found")
	}
	_routeTable := (listRouteTable.ResourceData.GetData()[0]).(ec2types.RouteTable)

	return &_routeTable, nil
}

func (b *BaseTask) GetSecurityGroup(throwErr ThrowErrorFlag) (*string, error) {
	listSecurityGroup := &ListSecurityGroup{BaseSecurityGroup: BaseSecurityGroup{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope}}}
	if err := listSecurityGroup.Execute(nil); err != nil {
		return nil, err
	}

	return listSecurityGroup.ResourceData.GetResourceArn(throwErr)
}

func (b *BaseTask) GetVpcItem(itemType string) (*string, error) {
	listVPC := &ListVPC{BaseVPC: BaseVPC{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope}}}
	if err := listVPC.Execute(nil); err != nil {
		return nil, err
	}

	return listVPC.GetVPCItem(itemType)
}

func (b *BaseTask) GetTransitGatewayID() (*string, error) {
	listTransitGateway := &ListTransitGateway{BaseTransitGateway: BaseTransitGateway{BaseTask: BaseTask{
		pexecutor:   b.pexecutor,
		clusterName: b.clusterName,
		clusterType: b.clusterType,
	}}}
	if err := listTransitGateway.Execute(nil); err != nil {
		return nil, err
	}

	return listTransitGateway.GetTransitGatewayID()
}

func (b *BaseTask) GetNLBArn(throwErr ThrowErrorFlag) (*string, error) {
	listNLB := &ListNLB{BaseNLB: BaseNLB{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope,
	}}}
	if err := listNLB.Execute(nil); err != nil {
		return nil, err
	}

	return listNLB.ResourceData.GetResourceArn(throwErr)
}

func (b *BaseTask) GetTargetGroupArn(throwErr ThrowErrorFlag) (*string, error) {
	listTargetGroup := &ListTargetGroup{BaseTargetGroup: BaseTargetGroup{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope,
	}}}
	if err := listTargetGroup.Execute(nil); err != nil {
		return nil, err
	}

	return listTargetGroup.ResourceData.GetResourceArn(throwErr)
}

func (b *BaseTask) GetElasticAddress(throwErr ThrowErrorFlag) (*string, error) {
	listElasticAddress := &ListElasticAddress{BaseElasticAddress: BaseElasticAddress{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope,
	}}}
	if err := listElasticAddress.Execute(nil); err != nil {
		return nil, err
	}

	return listElasticAddress.ResourceData.GetResourceArn(throwErr)
}

func (b *BaseTask) GetInternetGatewayID(throwErr ThrowErrorFlag) (*string, bool, error) {
	listInternetGateway := &ListInternetGateway{BaseInternetGateway: BaseInternetGateway{BaseTask: BaseTask{
		pexecutor:      b.pexecutor,
		clusterName:    b.clusterName,
		clusterType:    b.clusterType,
		subClusterType: b.subClusterType,
		scope:          b.scope,
	}}}
	if err := listInternetGateway.Execute(nil); err != nil {
		return nil, false, err
	}

	internetGatewayID, err := listInternetGateway.ResourceData.GetResourceArn(throwErr)
	if err != nil {
		return nil, false, err
	}

	isAttached, err := listInternetGateway.isAttachedToVPC()
	if err != nil {
		return nil, false, err
	}

	return internetGatewayID, isAttached, nil
}

// The route of the internet gateway is shared by public net and nat's internetegatewy

/*
componentName: alertmanager/cdc/grafana/pd/prometheus/tidb/tikv
*/
func (b *BaseTask) getTiDBComponent(componentName string) (*[]TiDBInstanceInfo, error) {
	tidbClusterInfos, err := b.getTiDBClusterInfo()
	if err != nil {
		return nil, err
	}

	var tidbInstancesInfo []TiDBInstanceInfo
	for _, instanceInfo := range (*tidbClusterInfos).Instances {
		if instanceInfo.Role == componentName {
			tidbInstancesInfo = append(tidbInstancesInfo, instanceInfo)
		}
	}

	return &tidbInstancesInfo, nil

}

func (b *BaseTask) waitUntilResouceDestroy(_interval, _timeout time.Duration, _readResource func() error) error {
	if _interval == 0 {
		_interval = 20 * time.Second
	}

	if _timeout == 0 {
		_timeout = 60 * time.Minute
	}

	timeout := time.After(_timeout)
	d := time.NewTicker(_interval)

	for {
		// Select statement
		select {
		case <-timeout:
			return errors.New("Timed out")
		case _ = <-d.C:
			if err := _readResource(); err != nil {
				return err
			}
			_data := b.ResourceData.GetData()
			if _data == nil || len(b.ResourceData.GetData()) == 0 {
				return nil
			}
		}
	}
}

func (b *BaseTask) waitUntilResouceAvailable(_interval, _timeout time.Duration, expectNum int, _readResource func() error) error {
	if _interval == 0 {
		_interval = 60 * time.Second
	}

	if _timeout == 0 {
		_timeout = 60 * time.Minute
	}

	timeout := time.After(_timeout)
	d := time.NewTicker(_interval)

	for {
		// Select statement
		select {
		case <-timeout:
			return errors.New("Timed out")
		case _ = <-d.C:
			if err := _readResource(); err != nil {
				return err
			}

			if len(b.ResourceData.GetData()) == expectNum {
				return nil
			}
		}
	}
}

// Move it to untils
// func WaitResourceUntilExpectState(_interval, _timeout time.Duration, _resourceStateCheck func() (bool, error)) error {
// 	timeout := time.After(_timeout)
// 	d := time.NewTicker(_interval)

// 	for {
// 		// Select statement
// 		select {
// 		case <-timeout:
// 			return errors.New("Timed out")
// 		case _ = <-d.C:
// 			resourceStateAsExpectFlag, err := _resourceStateCheck()
// 			if err != nil {
// 				return err
// 			}
// 			if resourceStateAsExpectFlag == true {
// 				return nil
// 			}
// 		}
// 	}
// }

// Deploy Redshift Instance
type RunCommonWS struct {
	wsExe    *ctxt.Executor
	packages *[]string
}

// Execute implements the Task interface
func (c *RunCommonWS) Execute(ctx context.Context) error {
	if _, _, err := (*c.wsExe).Execute(ctx, "mkdir -p /opt/scripts", true); err != nil {
		return err
	}

	if stdout, _, err := (*c.wsExe).Execute(ctx, "apt-get update -y", true); err != nil {
		if strings.Contains(string(stdout), "The following signatures couldn't be verified because the public key is not available:") {
			if _, _, err := (*c.wsExe).Execute(ctx, "apt-get install -y aptitude", true); err != nil {
				return err
			}

			if _, _, err := (*c.wsExe).Execute(ctx, "aptitude install -y debian-archive-keyring", true); err != nil {
				return err
			}

			if _, _, err := (*c.wsExe).Execute(ctx, "apt-get update -y", true); err != nil {
				return err
			}

		} else {
			return err
		}
	}

	if c.packages != nil {
		for _, _package := range *c.packages {
			if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("apt-get install -y %s", _package), true); err != nil {
				return err
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *RunCommonWS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *RunCommonWS) String() string {
	return fmt.Sprintf("Echo: Deploy common on workstation ")
}
