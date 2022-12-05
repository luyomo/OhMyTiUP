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
	"os"
	"path"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	astypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/embed"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"go.uber.org/zap"
)

type CreateWorkstation struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateWorkstation) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	tagProject := GetProject(ctx)
	tagOwner, err := GetCallerUser(ctx)
	if err != nil {
		return err
	}

	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=workstation\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}
	for _, reservation := range reservations.Reservations {
		for range reservation.Instances {
			return nil
		}
	}

	deviceStmt := ""
	if c.awsWSConfigs.VolumeSize > 0 {
		deviceStmt = fmt.Sprintf(" --block-device-mappings DeviceName=/dev/xvda,Ebs={VolumeSize=%d}", c.awsWSConfigs.VolumeSize)
	}
	command = fmt.Sprintf(
		"aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address "+
			"--key-name %s --security-group-ids %s --subnet-id %s %s "+
			"--tag-specifications \"ResourceType=instance,Tags=["+
			"  {Key=Name,Value=%s},"+
			"  {Key=Cluster,Value=%s},"+
			"  {Key=Type,Value=%s},"+
			"  {Key=Component,Value=workstation},"+
			"  {Key=Owner,Value=%s},"+
			"  {Key=Project,Value=%s}"+
			"]\"",
		c.awsWSConfigs.ImageId, c.awsWSConfigs.InstanceType,
		c.awsWSConfigs.KeyName, c.clusterInfo.publicSecurityGroupId, c.clusterInfo.publicSubnet, deviceStmt,
		clusterName,
		clusterType,
		c.subClusterType,
		tagOwner,
		tagProject,
	)

	zap.L().Debug("Command", zap.String("run-instances", command))
	stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateWorkstation) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateWorkstation) String() string {
	return fmt.Sprintf("Echo: Deploying Workstation")
}

/******************************************************************************/

type DestroyEC struct {
	pexecutor      *ctxt.Executor
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyEC) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	if len(reservations.Reservations) == 0 {
		return nil
	}

	for _, reservation := range reservations.Reservations {

		for _, instance := range reservation.Instances {

			command := fmt.Sprintf("aws ec2 terminate-instances --instance-ids %s", instance.InstanceId)
			stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
			if err != nil {
				return err
			}
		}
	}

	for idx := 0; idx < 50; idx++ {

		time.Sleep(5 * time.Second)
		command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("describe-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}

		var reservations Reservations
		if err = json.Unmarshal(stdout, &reservations); err != nil {
			zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
			return err
		}
		if len(reservations.Reservations) == 0 {
			break
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyEC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyEC) String() string {
	return fmt.Sprintf("Echo: Destroying EC")
}

/******************************************************************************/

type ListEC struct {
	pexecutor *ctxt.Executor
	tableECs  *[][]string
}

// Execute implements the Task interface
func (c *ListEC) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=instance-state-code,Values=0,16,64,80\"", clusterName, clusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	if len(reservations.Reservations) == 0 {
		return nil
	}

	for _, reservation := range reservations.Reservations {

		for _, instance := range reservation.Instances {
			componentName := "-"
			componentCluster := "-"
			for _, tagItem := range instance.Tags {
				if tagItem["Key"] == "Type" {
					componentCluster = tagItem["Value"]
				}
				if tagItem["Key"] == "Component" {
					componentName = tagItem["Value"]
				}
			}
			(*c.tableECs) = append(*c.tableECs, []string{
				componentName,
				componentCluster,
				instance.State.Name,
				instance.InstanceId,
				instance.InstanceType,
				instance.PrivateIpAddress,
				instance.PublicIpAddress,
				instance.ImageId,
			})
		}
	}
	sort.Sort(byComponentName(*c.tableECs))

	return nil
}

// Rollback implements the Task interface
func (c *ListEC) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListEC) String() string {
	return fmt.Sprintf("Echo: Listing EC")
}

func ListClusterEc2s(ctx context.Context, pexecutor ctxt.Executor, clusterName string) (*Reservations, error) {
	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\"  \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := pexecutor.Execute(ctx, command, false)
	if err != nil {
		return nil, err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil, err
	}
	return &reservations, nil
}

// ******************************************* Workspace
type DeployWS struct {
	pexecutor      *ctxt.Executor
	subClusterType string
	awsWSConfigs   *spec.AwsWSConfigs
}

// Execute implements the Task interface
func (c *DeployWS) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// 1. Get all the workstation nodes
	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
	if err != nil {
		return err
	}
	err = installWebSSH2(workstation, ctx)
	if err != nil {
		return err
	}

	return nil

	// 2. install docker/docker-compose/dnsutil
	if err := installPKGs(workstation, ctx, []string{"docker.io", "docker-compose", "dnsutils"}); err != nil {
		return err
	}

	// 3. add admin user into docker group
	if _, _, err := (*workstation).Execute(ctx, `gpasswd -a $USER docker`, true); err != nil {
		return err
	}

	nlb, err := getNLB(*c.pexecutor, ctx, clusterName, clusterType, c.subClusterType)
	if err != nil {
		return err
	}

	// 4. Deploy docker-compose file
	type TiDBCONN struct {
		TiDBHost string
		TiDBPort int
		TiDBUser string
		TiDBPass string
		TiDBName string
	}

	var tidbConn TiDBCONN
	tidbConn.TiDBHost = (*nlb).DNSName
	tidbConn.TiDBPort = 4000
	tidbConn.TiDBUser = "pdnsuser"
	tidbConn.TiDBPass = "pdnspass"
	tidbConn.TiDBName = "pdnsadmin"

	if _, _, err := (*workstation).Execute(ctx, `install -d -m 0755 -o admin -g admin /opt/pdns`, true); err != nil {
		return err
	}

	fdFile, err := os.Create("/tmp/pdns.yaml")
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp := path.Join("templates", "docker-compose", "pdns.yaml")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	if err := tmpl.Execute(fdFile, tidbConn); err != nil {
		return err
	}

	err = (*workstation).Transfer(ctx, "/tmp/pdns.yaml", "/opt/pdns/docker-compose.yaml", false, 0)
	if err != nil {
		return err
	}

	command := fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create user if not exists %s identified by \"%s\"'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBUser, tidbConn.TiDBPass)
	if _, _, err := (*workstation).Execute(ctx, command, true); err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBName)
	if _, _, err = (*workstation).Execute(ctx, command, true); err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'grant all on %s.* to %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBName, tidbConn.TiDBUser)
	if _, _, err = (*workstation).Execute(ctx, command, true); err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", "pdns")
	if _, _, err = (*workstation).Execute(ctx, command, true); err != nil {
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'grant all on %s.* to %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", "pdns", tidbConn.TiDBUser)
	if _, _, err = (*workstation).Execute(ctx, command, true); err != nil {
		return err
	}

	// 5. Add api info into pdns.conf
	fdFile, err = os.Create("/tmp/pdns.local.conf")
	if err != nil {
		return err
	}
	defer fdFile.Close()

	fp = path.Join("templates", "config", "pdns.local.conf.tpl")
	tpl, err = embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err = template.New("test").Parse(string(tpl))
	if err != nil {
		return err
	}

	type PdnsTpl struct {
		APIKey string
	}
	var pdnsTpl PdnsTpl
	pdnsTpl.APIKey = "sfkjdhsdfsfsddffffaddfh"

	if err := tmpl.Execute(fdFile, pdnsTpl); err != nil {
		return err
	}

	err = (*workstation).Transfer(ctx, "/tmp/pdns.local.conf", "/opt/pdns/pdns.local.conf", false, 0)
	if err != nil {
		return err
	}

	// 6. Create database for pdns
	if _, _, err := (*workstation).Execute(ctx, `docker-compose -f /opt/pdns/docker-compose.yaml up -d`, false, 300*time.Second); err != nil {
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (c *DeployWS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployWS) String() string {
	return fmt.Sprintf("Echo: Create CloudFormation ")
}

/*******************************************************************************/
type CreateEC2Nodes struct {
	pexecutor         *ctxt.Executor
	awsTopoConfigs    *spec.AwsNodeModal
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
	componentName     string
}

// Execute implements the Task interface
func (c *CreateEC2Nodes) Execute(ctx context.Context) error {
	/**********************************************************************
	 * 01. Pre process
	 **********************************************************************/
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// template name/auto scaling group name -> ohmytiup-tidb.clusterName.tidb.tikv
	combinedName := fmt.Sprintf("%s.%s.%s.%s", clusterType, clusterName, c.subClusterType, c.componentName)

	// Two tags to resource to show the project and owner
	tagProject := GetProject(ctx)       // Project name(tag): tidb-cluster
	tagOwner, err := GetCallerUser(ctx) // Owner(default): aws user
	if err != nil {
		return err
	}

	// Check whether the config is defined
	if c.awsTopoConfigs.Count == 0 {
		zap.L().Debug(fmt.Sprintf("There is no %s nodes to be configured", c.componentName))
		return nil
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)
	/***************************************************************************************************************
	 * 02. Check template existness
	 * Search template name
	 *     01. go to next step if the template has been created.
	 *     02. Create the template if it does not exist
	 ***************************************************************************************************************/

	describeLaunchTemplatesInput := &ec2.DescribeLaunchTemplatesInput{LaunchTemplateNames: []string{combinedName}}
	if _, err := client.DescribeLaunchTemplates(context.TODO(), describeLaunchTemplatesInput); err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			// fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() == "InvalidLaunchTemplateName.NotFoundException" {
				fmt.Print("Starting to create the launch template \n\n\n")
				c.CreateLaunchTemplate(client, &combinedName)
			} else {
				return err
			}
		} else {
			return err
		}
	}

	/***************************************************************************************************************
	 * 04. Check auto scaling group existness
	 ***************************************************************************************************************/
	clientASC := autoscaling.NewFromConfig(cfg)
	describeAutoScalingGroupsInput := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{combinedName},
	}

	describeAutoScalingGroups, err := clientASC.DescribeAutoScalingGroups(context.TODO(), describeAutoScalingGroupsInput)
	if err != nil {
		return err
	}

	// fmt.Printf("The auto scaling groups are <%#v> \n\n\n", len(describeAutoScalingGroups.AutoScalingGroups))

	/***************************************************************************************************************
	 * 05. Auto scaling generation
	 ***************************************************************************************************************/
	if len(describeAutoScalingGroups.AutoScalingGroups) == 0 {
		tags := []astypes.Tag{
			{Key: aws.String("Cluster"), Value: aws.String(clusterType)},       // ex: ohmytiup-tidb
			{Key: aws.String("Type"), Value: aws.String(c.subClusterType)},     // ex: tidb/oracle/workstation
			{Key: aws.String("Component"), Value: aws.String(c.componentName)}, // ex: tidb/tikv/pd
			{Key: aws.String("Name"), Value: aws.String(clusterName)},          // ex: clustertest
			{Key: aws.String("Owner"), Value: aws.String(tagOwner)},            // ex: aws-user
			{Key: aws.String("Project"), Value: aws.String(tagProject)},        // ex: clustertest
		}

		createAutoScalingGroupInput := &autoscaling.CreateAutoScalingGroupInput{
			AutoScalingGroupName: aws.String(combinedName),
			MaxSize:              aws.Int32(c.awsTopoConfigs.MaxSize),
			MinSize:              aws.Int32(c.awsTopoConfigs.MinSize),
			CapacityRebalance:    aws.Bool(true),
			DesiredCapacity:      aws.Int32(c.awsTopoConfigs.DesiredCapacity),
			LaunchTemplate: &astypes.LaunchTemplateSpecification{
				LaunchTemplateName: aws.String(combinedName),
				Version:            aws.String("$Latest"),
			},
			VPCZoneIdentifier: aws.String(strings.Join(c.clusterInfo.privateSubnets, ",")),
			Tags:              tags,
		}

		// Compatible to the old version of count config value
		if c.awsTopoConfigs.DesiredCapacity == 0 && c.awsTopoConfigs.Count > 0 {
			createAutoScalingGroupInput.MaxSize = aws.Int32(int32(c.awsTopoConfigs.Count))
			createAutoScalingGroupInput.MinSize = aws.Int32(int32(c.awsTopoConfigs.Count))
			createAutoScalingGroupInput.DesiredCapacity = aws.Int32(int32(c.awsTopoConfigs.Count))
		}

		// c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)]

		if _, err := clientASC.CreateAutoScalingGroup(context.TODO(), createAutoScalingGroupInput); err != nil {
			return err
		}
	}

	return nil
}

func (c *CreateEC2Nodes) CreateLaunchTemplate(client *ec2.Client, templateName *string) error {
	requestLaunchTemplateData := types.RequestLaunchTemplateData{}

	// 02. Storage template preparation
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
	requestLaunchTemplateData.EbsOptimized = aws.Bool(false)                                    // EbsOptimized flag, not support all the instance type
	requestLaunchTemplateData.ImageId = aws.String(c.awsGeneralConfigs.ImageId)                 // ImageID
	requestLaunchTemplateData.InstanceType = types.InstanceType(c.awsTopoConfigs.InstanceType)  // Instance Type
	requestLaunchTemplateData.KeyName = aws.String(c.awsGeneralConfigs.KeyName)                 // Key name
	requestLaunchTemplateData.SecurityGroupIds = []string{c.clusterInfo.privateSecurityGroupId} // security group

	// 03. Template data preparation
	createLaunchTemplateInput := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: templateName,
		LaunchTemplateData: &requestLaunchTemplateData,
	}

	// 04. Template generation
	_, err := client.CreateLaunchTemplate(context.TODO(), createLaunchTemplateInput)
	if err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateEC2Nodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateEC2Nodes) String() string {
	return fmt.Sprintf(fmt.Sprintf("Echo: Deploying %s Nodes ", c.componentName))
}

// ---------------------------------TiKV
/*******************************************************************************/
type CreateTiKVNodes struct {
	pexecutor         *ctxt.Executor
	awsTopoConfigs    *[]spec.AwsNodeModal
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
	componentName     string
}

// batch
//
//	-> zone
//
// online
//
//	-> zone
//
// Loop the topo structure, and find out all the instance.
//
//	Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: online, label/category: host01
//	Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: online, label/category: host02
//	Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: online, label/category: host03
//	Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: batch, label/category: host04
//	Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: batch, label/category: host05
//	Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: batch, label/category: host06
//
// Execute implements the Task interface
func (c *CreateTiKVNodes) Execute(ctx context.Context) error {

	// clusterName := ctx.Value("clusterName").(string)
	// clusterType := ctx.Value("clusterType").(string)
	fmt.Printf("Stoping here for TiKV node preparation \n\n\n")
	return errors.New("Test stop")

}

// Rollback implements the Task interface
func (c *CreateTiKVNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiKVNodes) String() string {
	return fmt.Sprintf(fmt.Sprintf("Echo: Deploying %s Nodes ", c.componentName))
}

// InstanceType:"c5.2xlarge", Count:3, VolumeSize:300, VolumeType:"gp3", Iops:3000
type EC2NodeConfig struct {
	InstanceType string
	Count        int
	VolumeSize   int
	VolumeType   string
	Iops         int

	Labels []map[string]string
}

// func ScanLabels(tikvLabels *[]spec.AwsTiKVLabel, tikvMachineTypes *[]spec.AwsTiKVMachineType) (*[]EC2NodeConfig, error) {
// 	if len(*tikvLabels) == 0 {
// 		return nil, nil
// 	}

// 	var ec2NodeConfigs []EC2NodeConfig

// 	for _, tikvLabel := range *tikvLabels {
// 		for _, tikvValue := range tikvLabel.Values {

// 			retEc2NodeConfig, err := ScanLabels(&tikvValue.Labels, tikvMachineTypes)
// 			if err != nil {
// 				return nil, err
// 			}
// 			if tikvValue.MachineType != "" {
// 				for _, machineType := range *tikvMachineTypes {
// 					if tikvValue.MachineType == machineType.Name {
// 						var ec2NodeConfig EC2NodeConfig
// 						ec2NodeConfig.InstanceType = machineType.ModalValue.InstanceType
// 						ec2NodeConfig.Count = machineType.ModalValue.Count
// 						ec2NodeConfig.VolumeSize = machineType.ModalValue.VolumeSize
// 						ec2NodeConfig.VolumeType = machineType.ModalValue.VolumeType
// 						ec2NodeConfig.Iops = machineType.ModalValue.Iops

// 						ec2NodeConfig.Labels = append(ec2NodeConfig.Labels, map[string]string{"label:" + tikvLabel.Name: tikvValue.Value})
// 						ec2NodeConfigs = append(ec2NodeConfigs, ec2NodeConfig)
// 					}
// 				}
// 			} else {
// 				for _, ec2NodeConfig := range *retEc2NodeConfig {
// 					ec2NodeConfig.Labels = append(ec2NodeConfig.Labels, map[string]string{"label:" + tikvLabel.Name: tikvValue.Value})
// 					ec2NodeConfigs = append(ec2NodeConfigs, ec2NodeConfig)
// 				}
// 			}

// 		}
// 	}

// 	return &ec2NodeConfigs, nil
// }

// *********** The package installation for parallel
// type MakeEC2Instance struct {
// 	ec2NodeConfig  EC2NodeConfig
// 	clusterName    string
// 	clusterType    string
// 	subClusterType string
// 	componentName  string
// 	tagOwner       string
// 	tagProject     string

// 	clusterInfo *ClusterInfo

// 	subnetID string

// 	awsTopoConfigs    *spec.AwsTiKVModal
// 	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
// }

// func (c *MakeEC2Instance) Execute(ctx context.Context) error {
// 	cfg, err := config.LoadDefaultConfig(context.TODO())
// 	if err != nil {
// 		return err
// 	}

// 	client := ec2.NewFromConfig(cfg)

// 	tags := []types.Tag{
// 		{
// 			Key:   aws.String("Cluster"),
// 			Value: aws.String(c.clusterType),
// 		},
// 		{
// 			Key:   aws.String("Type"),
// 			Value: aws.String(c.subClusterType), // tidb/oracle/workstation
// 		},
// 		{
// 			Key:   aws.String("Component"),
// 			Value: aws.String(c.componentName),
// 		},
// 		{
// 			Key:   aws.String("Name"),
// 			Value: aws.String(c.clusterName),
// 		},
// 		{
// 			Key:   aws.String("Owner"),
// 			Value: aws.String(c.tagOwner),
// 		},
// 		{
// 			Key:   aws.String("Project"),
// 			Value: aws.String(c.tagProject),
// 		},
// 	}

// 	for _, nodeLabel := range c.ec2NodeConfig.Labels {
// 		for labelKey, labelValue := range nodeLabel {
// 			tags = append(tags, types.Tag{
// 				Key:   aws.String(labelKey),
// 				Value: aws.String(labelValue),
// 			})
// 		}
// 	}

// 	tagSpecifications := []types.TagSpecification{
// 		{
// 			ResourceType: "instance",
// 			Tags:         tags,
// 		},
// 	}

// 	var runInstancesInput *ec2.RunInstancesInput

// 	if c.ec2NodeConfig.InstanceType != "" {
// 		runInstancesInput = &ec2.RunInstancesInput{
// 			ImageId:           aws.String(c.awsGeneralConfigs.ImageId),
// 			InstanceType:      types.InstanceType(c.ec2NodeConfig.InstanceType),
// 			TagSpecifications: tagSpecifications,
// 			KeyName:           aws.String(c.awsGeneralConfigs.KeyName),
// 			SecurityGroupIds:  []string{c.clusterInfo.privateSecurityGroupId},
// 			MaxCount:          aws.Int32(1),
// 			MinCount:          aws.Int32(1),
// 			SubnetId:          aws.String(c.subnetID),
// 		}

// 		if c.ec2NodeConfig.VolumeType != "" {
// 			blockDeviceMapping := types.BlockDeviceMapping{
// 				DeviceName: aws.String("/dev/xvda"),
// 				Ebs: &types.EbsBlockDevice{
// 					DeleteOnTermination: aws.Bool(true),
// 					Iops:                aws.Int32(int32(c.ec2NodeConfig.Iops)),
// 					VolumeSize:          aws.Int32(int32(c.ec2NodeConfig.VolumeSize)),
// 					VolumeType:          types.VolumeType(c.ec2NodeConfig.VolumeType),
// 				},
// 			}

// 			runInstancesInput.EbsOptimized = aws.Bool(true)
// 			runInstancesInput.BlockDeviceMappings = []types.BlockDeviceMapping{blockDeviceMapping}
// 		}
// 	} else {
// 		runInstancesInput = &ec2.RunInstancesInput{
// 			ImageId:           aws.String(c.awsGeneralConfigs.ImageId),
// 			InstanceType:      types.InstanceType((*c.awsTopoConfigs).InstanceType),
// 			TagSpecifications: tagSpecifications,
// 			KeyName:           aws.String((*c.awsGeneralConfigs).KeyName),
// 			SecurityGroupIds:  []string{c.clusterInfo.privateSecurityGroupId},
// 			MaxCount:          aws.Int32(1),
// 			MinCount:          aws.Int32(1),
// 			SubnetId:          aws.String(c.subnetID),
// 		}

// 		if (*c.awsTopoConfigs).VolumeType != "" {
// 			blockDeviceMapping := types.BlockDeviceMapping{
// 				DeviceName: aws.String("/dev/xvda"),
// 				Ebs: &types.EbsBlockDevice{
// 					DeleteOnTermination: aws.Bool(true),
// 					Iops:                aws.Int32(int32((*c.awsTopoConfigs).Iops)),
// 					VolumeSize:          aws.Int32(int32((*c.awsTopoConfigs).VolumeSize)),
// 					VolumeType:          types.VolumeType((*c.awsTopoConfigs).VolumeType),
// 				},
// 			}

// 			runInstancesInput.EbsOptimized = aws.Bool(true)
// 			runInstancesInput.BlockDeviceMappings = []types.BlockDeviceMapping{blockDeviceMapping}
// 		}
// 	}

// 	retInstance, err := client.RunInstances(context.TODO(), runInstancesInput)
// 	if err != nil {
// 		return err
// 	}

// 	isRunning, err := WaitInstanceRunnung(ctx, *(retInstance.Instances[0].InstanceId))
// 	if err != nil {
// 		return err
// 	}

// 	if isRunning == false {
// 		return errors.New("Failed to wait the instance to become running state")
// 	}
// 	zap.L().Debug("create EC2 nodes by SDK info", zap.String("clusterName", c.clusterName), zap.String("clusterType", c.clusterType), zap.String("subClusterType", c.subClusterType), zap.String("componentName", c.componentName), zap.String("tagOwner", c.tagOwner), zap.String("tagProject", c.tagProject), zap.String("subnetID", c.subnetID), zap.String("awsTopoConfigs", fmt.Sprintf("%+v", c.awsTopoConfigs)), zap.String("awsGeneralConfigs", fmt.Sprintf("%+v", c.awsGeneralConfigs)))
// 	zap.L().Debug(fmt.Sprintf("EC2 tags : %+v", tagSpecifications))
// 	return nil
// }
// func (c *MakeEC2Instance) Rollback(ctx context.Context) error {
// 	return ErrUnsupportedRollback
// }

// func (c *MakeEC2Instance) String() string {
// 	return fmt.Sprintf("Echo: Make one EC2 Instance")
// }

// func FetchTiKVInstances(ctx context.Context, clusterName, clusterType, subClusterType, componentName string, funcTags func([]types.Tag) error) error {
// 	cfg, err := config.LoadDefaultConfig(context.TODO())
// 	if err != nil {
// 		return err
// 	}

// 	client := ec2.NewFromConfig(cfg)

// 	var filters []types.Filter
// 	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{clusterName}})        // TiDBTestNode
// 	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{clusterType}})     // TiDB/TiDB2Kafka2PG
// 	filters = append(filters, types.Filter{Name: aws.String("tag:Component"), Values: []string{componentName}}) // TiKV/TiDB/PD
// 	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{subClusterType}})
// 	filters = append(filters, types.Filter{Name: aws.String("instance-state-name"), Values: []string{"running", "pending", "stopping", "stopped"}})

// 	input := &ec2.DescribeInstancesInput{Filters: filters}

// 	result, err := client.DescribeInstances(context.TODO(), input)
// 	if err != nil {
// 		return err
// 	}

// 	for _, reservation := range result.Reservations {
// 		for _, instance := range reservation.Instances {
// 			funcTags(instance.Tags)
// 		}
// 	}

// 	return nil
// }

// func WaitInstanceRunnung(ctx context.Context, instanceId string) (bool, error) {
// 	cfg, err := config.LoadDefaultConfig(context.TODO())
// 	if err != nil {
// 		return false, err
// 	}

// 	client := ec2.NewFromConfig(cfg)

// 	var filters []types.Filter

// 	filters = append(filters, types.Filter{
// 		Name:   aws.String("instance-id"),
// 		Values: []string{instanceId},
// 	})

// 	filters = append(filters, types.Filter{
// 		Name:   aws.String("instance-state-name"),
// 		Values: []string{"running"},
// 	})

// 	input := &ec2.DescribeInstancesInput{Filters: filters}

// 	for idx := 0; idx < 50; idx++ {
// 		time.Sleep(3 * time.Second)
// 		result, err := client.DescribeInstances(context.TODO(), input)
// 		if err != nil {
// 			return false, err
// 		}

// 		if len(result.Reservations) > 0 && len(result.Reservations[0].Instances) > 0 {
// 			return true, nil
// 		}
// 	}

// 	return false, nil
// }

type ListAllAwsEC2 struct {
	pexecutor *ctxt.Executor
	tableEC2  *[][]string
}

func (c *ListAllAwsEC2) Execute(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter

	filters = append(filters, types.Filter{
		Name:   aws.String("instance-state-name"),
		Values: []string{"running", "terminated"},
	})

	input := &ec2.DescribeInstancesInput{Filters: filters}

	result, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return err
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			(*c.tableEC2) = append(*c.tableEC2, []string{
				string(instance.InstanceType),
				string(*instance.KeyName),
				string(instance.LaunchTime.String()),
				string(instance.State.Name),
			})
		}

	}

	return nil
}

// Rollback implements the Task interface
func (c *ListAllAwsEC2) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListAllAwsEC2) String() string {
	return fmt.Sprintf("Echo: Deploying Workstation")
}

func FetchInstances(ctx context.Context, tags *map[string]string, _func *func(types.Instance) error) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	for _key, _value := range *tags {
		filters = append(filters, types.Filter{
			Name:   aws.String("tag:" + _key),
			Values: []string{_value},
		})
	}

	filters = append(filters, types.Filter{
		Name:   aws.String("instance-state-name"),
		Values: []string{"running", "pending", "stopping", "stopped"},
	})

	input := &ec2.DescribeInstancesInput{Filters: filters}

	result, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return err
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			(*_func)(instance)
			//			_arrIPs = append(_arrIPs, *instance.PrivateIpAddress)
		}
	}

	return nil
}
