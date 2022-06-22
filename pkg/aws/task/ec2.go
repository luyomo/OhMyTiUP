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
	"fmt"
	"os"
	"path"
	"sort"
	"text/template"
	"time"

	"github.com/luyomo/tisample/embed"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
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
	command = fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=workstation}]\"", c.awsWSConfigs.ImageId, c.awsWSConfigs.InstanceType, c.awsWSConfigs.KeyName, c.clusterInfo.publicSecurityGroupId, c.clusterInfo.publicSubnet, deviceStmt, clusterName, clusterType, c.subClusterType)
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

//******************************************* Workspace
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
	stdout, _, err := (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The ut data is <%s> \n\n\n", string(stdout))
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBName)
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The ut data is <%s> \n\n\n", string(stdout))
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'grant all on %s.* to %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", tidbConn.TiDBName, tidbConn.TiDBUser)
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The ut data is <%s> \n\n\n", string(stdout))
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'create database if not exists %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", "pdns")
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The ut data is <%s> \n\n\n", string(stdout))
		return err
	}

	command = fmt.Sprintf(`mysql -h %s -P %d -u %s -e 'grant all on %s.* to %s'`, tidbConn.TiDBHost, tidbConn.TiDBPort, "root", "pdns", tidbConn.TiDBUser)
	stdout, _, err = (*workstation).Execute(ctx, command, true)
	if err != nil {
		fmt.Printf("The command is <%s> \n\n\n", command)
		fmt.Printf("The ut data is <%s> \n\n\n", string(stdout))
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
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// Check whether the config is defined
	if c.awsTopoConfigs.Count == 0 {
		zap.L().Debug(fmt.Sprintf("There is no %s nodes to be configured", c.componentName))
		return nil
	}

	// Query against AWS whether the nodes have been generated by script
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType, c.componentName)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	// Parse the return from aws console api
	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	// Count the number of the EC2 nodes
	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	// Loop until generating all the required nodes
	for _idx := 0; _idx < c.awsTopoConfigs.Count; _idx++ {
		if _idx < existsNodes {
			continue
		}

		// If the volume size is specified, add the below statement
		deviceStmt := ""
		if c.awsTopoConfigs.VolumeSize > 0 {
			deviceStmt = fmt.Sprintf(" --block-device-mappings \"DeviceName=/dev/xvda,Ebs={VolumeSize=%d,VolumeType=%s,Iops=%d}\"", c.awsTopoConfigs.VolumeSize, c.awsTopoConfigs.VolumeType, c.awsTopoConfigs.Iops)
		}

		// Start the instance
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --ebs-optimized --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=%s}]\"", c.awsGeneralConfigs.ImageId, c.awsTopoConfigs.InstanceType, c.awsGeneralConfigs.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt, clusterName, clusterType, c.subClusterType, c.componentName)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
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
