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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	tagEmail := ctx.Value("tagEmail").(string)
	tagProject := ctx.Value("tagProject").(string)

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
			"  {Key=Email,Value=%s},"+
			"  {Key=Project,Value=%s}"+
			"]\"",
		c.awsWSConfigs.ImageId, c.awsWSConfigs.InstanceType,
		c.awsWSConfigs.KeyName, c.clusterInfo.publicSecurityGroupId, c.clusterInfo.publicSubnet, deviceStmt,
		clusterName,
		clusterType,
		c.subClusterType,
		tagEmail,
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
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	tagEmail := ctx.Value("tagEmail").(string)
	tagProject := ctx.Value("tagProject").(string)

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
		command := fmt.Sprintf(
			"aws ec2 run-instances --count 1 --image-id %s --ebs-optimized "+
				"--instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s "+
				"--tag-specifications \"ResourceType=instance,Tags=["+
				"  {Key=Name,Value=%s},"+
				"  {Key=Cluster,Value=%s},"+
				"  {Key=Type,Value=%s},"+
				"  {Key=Component,Value=%s},"+
				"  {Key=Email,Value=%s},"+
				"  {Key=Project,Value=%s}"+
				"]\"",
			c.awsGeneralConfigs.ImageId,
			c.awsTopoConfigs.InstanceType, c.awsGeneralConfigs.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt,
			clusterName,
			clusterType,
			c.subClusterType,
			c.componentName,
			tagEmail,
			tagProject,
		)
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

// ---------------------------------TiKV
/*******************************************************************************/
type CreateTiKVNodes struct {
	pexecutor         *ctxt.Executor
	awsTopoConfigs    *spec.AwsTiKVModal
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
	componentName     string
}

//
// batch
//   -> zone
// online
//   -> zone
// Loop the topo structure, and find out all the instance.
//    Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: online, label/category: host01
//    Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: online, label/category: host02
//    Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: online, label/category: host03
//    Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: batch, label/category: host04
//    Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: batch, label/category: host05
//    Name: test-name,  Cluster: ohmytiup-tidb, component: tikv, label/category: batch, label/category: host06
// Execute implements the Task interface
func (c *CreateTiKVNodes) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)
	tagEmail := ctx.Value("tagEmail").(string)
	tagProject := ctx.Value("tagProject").(string)

	ec2NodeConfigs, err := ScanLabels(&(*c.awsTopoConfigs).Labels, &(*c.awsTopoConfigs).ModalTypes)
	if err != nil {
		return err
	}

	// Define the function to remove or reduce the nodes from label definition if it has been created in the AWS.
	funFilter := func(tags []types.Tag) error {

		if ec2NodeConfigs != nil {
			for idx, nodeLabel := range *ec2NodeConfigs {
				for _, label := range nodeLabel.Labels {
					var arrayLabel []string
					for labelKey, labelValue := range label {
						arrayLabel = append(arrayLabel, labelKey+"="+labelValue)
					}
					strLabel := strings.Join(arrayLabel, ",")

					var arrayNodeLabel []string
					for _, tag := range tags {
						if strings.Contains(*(tag.Key), "label:") {
							arrayNodeLabel = append(arrayNodeLabel, *(tag.Key)+"="+*(tag.Value))
						}
					}

					if strings.Join(arrayNodeLabel, ",") == strLabel {
						if (*ec2NodeConfigs)[idx].Count > 0 {
							((*ec2NodeConfigs)[idx]).Count = ((*ec2NodeConfigs)[idx]).Count - 1
						}
						return nil
					}

				}
			}
		}

		for _, tag := range tags {
			if strings.Contains(*(tag.Key), "label:") {
				return nil
			}
		}
		if (*c.awsTopoConfigs).Count > 0 {
			(*c.awsTopoConfigs).Count = (*c.awsTopoConfigs).Count - 1
		}

		return nil
	}

	// Run filtering. If the instances have been created in the AWS, reduce the number from the definition
	FetchTiKVInstances(ctx, clusterName, clusterType, c.subClusterType, c.componentName, funFilter)

	// Define the tasks to generate the instance
	var generateInstancesTask []Task

	if ec2NodeConfigs != nil {
		for _, ec2NodeConfig := range *ec2NodeConfigs {
			for idx := 1; idx <= ec2NodeConfig.Count; idx++ {
				makeEc2Instance := &MakeEC2Instance{
					ec2NodeConfig:     ec2NodeConfig,
					clusterName:       clusterName,
					clusterType:       clusterType,
					tagEmail:          tagEmail,
					tagProject:        tagProject,
					subClusterType:    c.subClusterType,
					componentName:     c.componentName,
					clusterInfo:       c.clusterInfo,
					awsGeneralConfigs: c.awsGeneralConfigs,
					subnetID:          c.clusterInfo.privateSubnets[idx%len(c.clusterInfo.privateSubnets)], // Todo. How to decide the proper value
				}
				generateInstancesTask = append(generateInstancesTask, makeEc2Instance)
			}
		}
	}

	for idx := 0; idx < (*c.awsTopoConfigs).Count; idx++ {
		makeEc2Instance := &MakeEC2Instance{
			clusterName:       clusterName,
			clusterType:       clusterType,
			tagEmail:          tagEmail,
			tagProject:        tagProject,
			subClusterType:    c.subClusterType,
			componentName:     c.componentName,
			clusterInfo:       c.clusterInfo,
			awsTopoConfigs:    c.awsTopoConfigs,
			awsGeneralConfigs: c.awsGeneralConfigs,
			subnetID:          c.clusterInfo.privateSubnets[idx%len(c.clusterInfo.privateSubnets)], // Todo. How to decide the proper value
		}
		generateInstancesTask = append(generateInstancesTask, makeEc2Instance)
	}

	parallelExe := Parallel{ignoreError: false, inner: generateInstancesTask}
	if err := parallelExe.Execute(ctx); err != nil {
		return err
	}

	return nil
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

func ScanLabels(tikvLabels *[]spec.AwsTiKVLabel, tikvMachineTypes *[]spec.AwsTiKVMachineType) (*[]EC2NodeConfig, error) {
	if len(*tikvLabels) == 0 {
		return nil, nil
	}

	var ec2NodeConfigs []EC2NodeConfig

	for _, tikvLabel := range *tikvLabels {
		for _, tikvValue := range tikvLabel.Values {

			retEc2NodeConfig, err := ScanLabels(&tikvValue.Labels, tikvMachineTypes)
			if err != nil {
				return nil, err
			}
			if tikvValue.MachineType != "" {
				for _, machineType := range *tikvMachineTypes {
					if tikvValue.MachineType == machineType.Name {
						var ec2NodeConfig EC2NodeConfig
						ec2NodeConfig.InstanceType = machineType.ModalValue.InstanceType
						ec2NodeConfig.Count = machineType.ModalValue.Count
						ec2NodeConfig.VolumeSize = machineType.ModalValue.VolumeSize
						ec2NodeConfig.VolumeType = machineType.ModalValue.VolumeType
						ec2NodeConfig.Iops = machineType.ModalValue.Iops

						ec2NodeConfig.Labels = append(ec2NodeConfig.Labels, map[string]string{"label:" + tikvLabel.Name: tikvValue.Value})
						ec2NodeConfigs = append(ec2NodeConfigs, ec2NodeConfig)
					}
				}
			} else {
				for _, ec2NodeConfig := range *retEc2NodeConfig {
					ec2NodeConfig.Labels = append(ec2NodeConfig.Labels, map[string]string{"label:" + tikvLabel.Name: tikvValue.Value})
					ec2NodeConfigs = append(ec2NodeConfigs, ec2NodeConfig)
				}
			}

		}
	}

	return &ec2NodeConfigs, nil
}

// *********** The package installation for parallel
type MakeEC2Instance struct {
	ec2NodeConfig  EC2NodeConfig
	clusterName    string
	clusterType    string
	subClusterType string
	componentName  string
	tagEmail       string
	tagProject     string

	clusterInfo *ClusterInfo

	subnetID string

	awsTopoConfigs *spec.AwsTiKVModal
	//	awsTopoConfigs    *spec.AwsNodeModal
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
}

func (c *MakeEC2Instance) Execute(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	tags := []types.Tag{
		{
			Key:   aws.String("Cluster"),
			Value: aws.String(c.clusterType),
		},
		{
			Key:   aws.String("Type"),
			Value: aws.String(c.subClusterType), // tidb/oracle/workstation
		},
		{
			Key:   aws.String("Component"),
			Value: aws.String(c.componentName),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String(c.clusterName),
		},
		{
			Key:   aws.String("Email"),
			Value: aws.String(c.tagEmail),
		},
		{
			Key:   aws.String("Project"),
			Value: aws.String(c.tagProject),
		},
	}

	for _, nodeLabel := range c.ec2NodeConfig.Labels {
		for labelKey, labelValue := range nodeLabel {
			tags = append(tags, types.Tag{
				Key:   aws.String(labelKey),
				Value: aws.String(labelValue),
			})
		}
	}

	tagSpecifications := []types.TagSpecification{
		{
			ResourceType: "instance",
			Tags:         tags,
		},
	}

	var runInstancesInput *ec2.RunInstancesInput

	if c.ec2NodeConfig.InstanceType != "" {
		runInstancesInput = &ec2.RunInstancesInput{
			ImageId:           aws.String(c.awsGeneralConfigs.ImageId),
			InstanceType:      types.InstanceType(c.ec2NodeConfig.InstanceType),
			TagSpecifications: tagSpecifications,
			KeyName:           aws.String(c.awsGeneralConfigs.KeyName),
			SecurityGroupIds:  []string{c.clusterInfo.privateSecurityGroupId},
			MaxCount:          aws.Int32(1),
			MinCount:          aws.Int32(1),
			SubnetId:          aws.String(c.subnetID),
		}

		if c.ec2NodeConfig.VolumeType != "" {
			blockDeviceMapping := types.BlockDeviceMapping{
				DeviceName: aws.String("/dev/xvda"),
				Ebs: &types.EbsBlockDevice{
					DeleteOnTermination: aws.Bool(true),
					Iops:                aws.Int32(int32(c.ec2NodeConfig.Iops)),
					VolumeSize:          aws.Int32(int32(c.ec2NodeConfig.VolumeSize)),
					VolumeType:          types.VolumeType(c.ec2NodeConfig.VolumeType),
				},
			}

			runInstancesInput.EbsOptimized = aws.Bool(true)
			runInstancesInput.BlockDeviceMappings = []types.BlockDeviceMapping{blockDeviceMapping}
		}
	} else {
		runInstancesInput = &ec2.RunInstancesInput{
			ImageId:           aws.String(c.awsGeneralConfigs.ImageId),
			InstanceType:      types.InstanceType((*c.awsTopoConfigs).InstanceType),
			TagSpecifications: tagSpecifications,
			KeyName:           aws.String((*c.awsGeneralConfigs).KeyName),
			SecurityGroupIds:  []string{c.clusterInfo.privateSecurityGroupId},
			MaxCount:          aws.Int32(1),
			MinCount:          aws.Int32(1),
			SubnetId:          aws.String(c.subnetID),
		}

		if (*c.awsTopoConfigs).VolumeType != "" {
			blockDeviceMapping := types.BlockDeviceMapping{
				DeviceName: aws.String("/dev/xvda"),
				Ebs: &types.EbsBlockDevice{
					DeleteOnTermination: aws.Bool(true),
					Iops:                aws.Int32(int32((*c.awsTopoConfigs).Iops)),
					VolumeSize:          aws.Int32(int32((*c.awsTopoConfigs).VolumeSize)),
					VolumeType:          types.VolumeType((*c.awsTopoConfigs).VolumeType),
				},
			}

			runInstancesInput.EbsOptimized = aws.Bool(true)
			runInstancesInput.BlockDeviceMappings = []types.BlockDeviceMapping{blockDeviceMapping}
		}
	}

	retInstance, err := client.RunInstances(context.TODO(), runInstancesInput)
	if err != nil {
		return err
	}

	isRunning, err := WaitInstanceRunnung(ctx, *(retInstance.Instances[0].InstanceId))
	if err != nil {
		return err
	}

	if isRunning == false {
		return errors.New("Failed to wait the instance to become running state")
	}

	return nil
}
func (c *MakeEC2Instance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

func (c *MakeEC2Instance) String() string {
	return fmt.Sprintf("Echo: Make one EC2 Instance")
}

func FetchTiKVInstances(ctx context.Context, clusterName, clusterType, subClusterType, componentName string, funcTags func([]types.Tag) error) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	filters = append(filters, types.Filter{
		Name:   aws.String("tag:Name"),
		Values: []string{clusterName},
	})

	filters = append(filters, types.Filter{
		Name:   aws.String("tag:Cluster"),
		Values: []string{clusterType},
	})

	filters = append(filters, types.Filter{
		Name:   aws.String("tag:Component"),
		Values: []string{componentName},
	})

	filters = append(filters, types.Filter{
		Name:   aws.String("tag:Type"),
		Values: []string{subClusterType},
	})

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
			funcTags(instance.Tags)
		}

	}

	return nil
}

func WaitInstanceRunnung(ctx context.Context, instanceId string) (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return false, err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter

	filters = append(filters, types.Filter{
		Name:   aws.String("instance-id"),
		Values: []string{instanceId},
	})

	filters = append(filters, types.Filter{
		Name:   aws.String("instance-state-name"),
		Values: []string{"running"},
	})

	input := &ec2.DescribeInstancesInput{Filters: filters}

	for idx := 0; idx < 50; idx++ {
		time.Sleep(3 * time.Second)
		result, err := client.DescribeInstances(context.TODO(), input)
		if err != nil {
			return false, err
		}

		if len(result.Reservations) > 0 && len(result.Reservations[0].Instances) > 0 {
			return true, nil
		}
	}

	return false, nil
}

type ListAllAwsEC2 struct {
	pexecutor *ctxt.Executor
	tableEC2  *[][]string
}

//func ListAllEC2(ctx context.Context, clusterName, clusterType, subClusterType, componentName string) error {
func (c *ListAllAwsEC2) Execute(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := ec2.NewFromConfig(cfg)

	var filters []types.Filter
	// filters = append(filters, types.Filter{
	// 	Name:   aws.String("tag:Name"),
	// 	Values: []string{clusterName},
	// })

	// filters = append(filters, types.Filter{
	// 	Name:   aws.String("tag:Cluster"),
	// 	Values: []string{clusterType},
	// })

	// filters = append(filters, types.Filter{
	// 	Name:   aws.String("tag:Component"),
	// 	Values: []string{componentName},
	// })

	// filters = append(filters, types.Filter{
	// 	Name:   aws.String("tag:Type"),
	// 	Values: []string{subClusterType},
	// })

	filters = append(filters, types.Filter{
		Name: aws.String("instance-state-name"),
		//		Values: []string{"running", "pending", "stopping", "stopped"},
		Values: []string{"running", "terminated"},
	})

	input := &ec2.DescribeInstancesInput{Filters: filters}

	result, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return err
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			fmt.Printf("The data is <%#v> \n", instance.Tags)
			fmt.Printf("The data is <%#v> \n", instance.InstanceType)

			(*c.tableEC2) = append(*c.tableEC2, []string{
				// instance.State.Name,
				// instance.InstanceId,
				string(instance.InstanceType),
				string(*instance.KeyName),
				string(instance.LaunchTime.String()),
				string(instance.State.Name),
				// string(instance.InstanceType),
				// string(instance.InstanceType),
				// string(instance.InstanceType),
				// instance.PrivateIpAddress,
				// instance.PublicIpAddress,
				// instance.ImageId,
			})

			// (*c.tableEC2) = append(*c.tableEC2, []string{
			// 	"test",
			// 	"test",
			// 	instance.State.Name,
			// 	instance.InstanceId,
			// 	instance.InstanceType,
			// 	instance.PrivateIpAddress,
			// 	instance.PublicIpAddress,
			// 	instance.ImageId,
			// })

			fmt.Printf("The instance here is <%#v> \n\n\n", instance)
			// funcTags(instance.Tags)
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
