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
	"sort"
	"time"

	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"go.uber.org/zap"
)

type CreateDMNodes struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMNodes) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	if c.awsTopoConfigs.DM.Count == 0 {
		zap.L().Debug("There is no DM nodes to be configured")
		return nil
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=dm\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	//	fmt.Printf("The existed nodes are <%#v> \n\n\n", reservations.Reservations)
	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	for _idx := 0; _idx < c.awsTopoConfigs.DM.Count; _idx++ {
		if _idx < existsNodes {
			continue
		}
		deviceStmt := ""
		if c.awsTopoConfigs.DM.VolumeSize > 0 {
			deviceStmt = fmt.Sprintf(" --block-device-mappings DeviceName=/dev/xvda,Ebs={VolumeSize=%d}", c.awsTopoConfigs.DM.VolumeSize)
		}
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=dm}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.DM.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt, clusterName, clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateDMNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateDMNodes) String() string {
	return fmt.Sprintf("Echo: Deploying DM Nodes ")
}

/******************************************************************************/

type CreatePDNodes struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreatePDNodes) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	if c.awsTopoConfigs.PD.Count == 0 {
		zap.L().Debug("There is no PD nodes to be configured")
		return nil
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=pd\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
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
	if len(reservations.Reservations) > 0 && len(reservations.Reservations[0].Instances) >= c.awsTopoConfigs.PD.Count {
		zap.L().Info("No need to make PD nodes", zap.String("PD instances", reservations.String()), zap.Int("# requested nodes", c.awsTopoConfigs.PD.Count))
		return nil
	}
	zap.L().Debug("Instances", zap.String("reservations", reservations.String()))
	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	for _idx := 0; _idx < c.awsTopoConfigs.PD.Count; _idx++ {
		if _idx < existsNodes {
			continue
		}
		deviceStmt := ""
		if c.awsTopoConfigs.PD.VolumeSize > 0 {
			deviceStmt = fmt.Sprintf(" --block-device-mappings DeviceName=/dev/xvda,Ebs={VolumeSize=%d}", c.awsTopoConfigs.PD.VolumeSize)
		}

		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=pd}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.PD.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt, clusterName, clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreatePDNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreatePDNodes) String() string {
	return fmt.Sprintf("Echo: Deploying PD Nodes")
}

/******************************************************************************/

type CreateTiCDCNodes struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiCDCNodes) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	if c.awsTopoConfigs.TiCDC.Count == 0 {
		zap.L().Debug("There is no TiCDC nodes to be configured")
		return nil
	}

	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=ticdc\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	//	fmt.Printf("The existed nodes are <%#v> \n\n\n", reservations.Reservations)
	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	for _idx := 0; _idx < c.awsTopoConfigs.TiCDC.Count; _idx++ {
		if _idx < existsNodes {
			continue
		}
		deviceStmt := ""
		if c.awsTopoConfigs.TiCDC.VolumeSize > 0 {
			deviceStmt = fmt.Sprintf(" --block-device-mappings DeviceName=/dev/xvda,Ebs={VolumeSize=%d}", c.awsTopoConfigs.TiCDC.VolumeSize)
		}
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=ticdc}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiCDC.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt, clusterName, clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateTiCDCNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiCDCNodes) String() string {
	return fmt.Sprintf("Echo: Deploying TiCDC Nodes ")
}

/******************************************************************************/

type CreateTiDBNodes struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiDBNodes) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	if c.awsTopoConfigs.TiDB.Count == 0 {
		zap.L().Debug("There is no TiDB nodes to be configured")
		return nil
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=tidb\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	for _idx := 0; _idx < c.awsTopoConfigs.TiDB.Count; _idx++ {
		if _idx < existsNodes {
			continue
		}
		deviceStmt := ""
		if c.awsTopoConfigs.TiDB.VolumeSize > 0 {
			deviceStmt = fmt.Sprintf(" --block-device-mappings DeviceName=/dev/xvda,Ebs={VolumeSize=%d}", c.awsTopoConfigs.TiDB.VolumeSize)
		}
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=tidb}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiDB.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt, clusterName, clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateTiDBNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiDBNodes) String() string {
	return fmt.Sprintf("Echo: Deploying TiDB Nodes ")
}

/******************************************************************************/

type CreateTiKVNodes struct {
	pexecutor      *ctxt.Executor
	awsTopoConfigs *spec.AwsTopoConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiKVNodes) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	if c.awsTopoConfigs.TiKV.Count == 0 {
		zap.L().Debug("There is no TiKV nodes to be configured")
		return nil
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=tikv\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, c.subClusterType)
	stdout, _, err := (*c.pexecutor).Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}

	existsNodes := 0
	for _, reservation := range reservations.Reservations {
		existsNodes = existsNodes + len(reservation.Instances)
	}

	for _idx := 0; _idx < c.awsTopoConfigs.TiKV.Count; _idx++ {
		if _idx < existsNodes {
			continue
		}
		deviceStmt := ""
		if c.awsTopoConfigs.TiKV.VolumeSize > 0 {
			deviceStmt = fmt.Sprintf(" --block-device-mappings DeviceName=/dev/xvda,Ebs={VolumeSize=%d}", c.awsTopoConfigs.TiKV.VolumeSize)
		}
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=tikv}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiKV.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], deviceStmt, clusterName, clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = (*c.pexecutor).Execute(ctx, command, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateTiKVNodes) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateTiKVNodes) String() string {
	return fmt.Sprintf("Echo: Deploying TiKV Nodes ")
}

/******************************************************************************/

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
		for _, _ = range reservation.Instances {
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
