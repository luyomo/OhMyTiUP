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
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/executor"
	"go.uber.org/zap"
	"time"
)

type CreateDMNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateDMNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	if c.awsTopoConfigs.DM.Count == 0 {
		zap.L().Debug("There is no DM nodes to be configured")
		return err
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=dm\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := local.Execute(ctx, command, false)
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

	for _idx := 0; _idx < c.awsTopoConfigs.DM.Count-existsNodes; _idx++ {
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=dm}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.DM.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], c.clusterName, c.clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type CreatePDNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreatePDNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	if c.awsTopoConfigs.PD.Count == 0 {
		zap.L().Debug("There is no PD nodes to be configured")
		return err
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=pd\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := local.Execute(ctx, command, false)
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

	for _idx := 0; _idx < c.awsTopoConfigs.PD.Count-existsNodes; _idx++ {
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=pd}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.PD.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], c.clusterName, c.clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type CreateTiCDCNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiCDCNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	if c.awsTopoConfigs.TiCDC.Count == 0 {
		zap.L().Debug("There is no TiCDC nodes to be configured")
		return err
	}

	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=ticdc\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instance", command))
	stdout, _, err := local.Execute(ctx, command, false)
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

	for _idx := 0; _idx < c.awsTopoConfigs.TiCDC.Count-existsNodes; _idx++ {
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=ticdc}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiCDC.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], c.clusterName, c.clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type CreateTiDBNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiDBNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	if c.awsTopoConfigs.TiDB.Count == 0 {
		zap.L().Debug("There is no TiDB nodes to be configured")
		return err
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=tidb\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := local.Execute(ctx, command, false)
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

	for _idx := 0; _idx < c.awsTopoConfigs.TiDB.Count-existsNodes; _idx++ {
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=tidb}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiDB.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], c.clusterName, c.clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type CreateTiKVNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateTiKVNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}

	if c.awsTopoConfigs.TiKV.Count == 0 {
		zap.L().Debug("There is no TiKV nodes to be configured")
		return err
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=tikv\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	stdout, _, err := local.Execute(ctx, command, false)
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

	for _idx := 0; _idx < c.awsTopoConfigs.TiKV.Count-existsNodes; _idx++ {
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=tikv}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiKV.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], c.clusterName, c.clusterType, c.subClusterType)
		//command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --block-device-mappings DeviceName=/dev/sdh,Ebs={DeleteOnTermination=true,VolumeSize=100,VolumeType=gp2,Encrypted=false} --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=tikv}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.TiKV.InstanceType, c.awsTopoConfigs.General.KeyName, c.clusterInfo.privateSecurityGroupId, c.clusterInfo.privateSubnets[_idx%len(c.clusterInfo.privateSubnets)], c.clusterName, c.clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type CreateWorkstation struct {
	user           string
	host           string
	awsWSConfigs   *spec.AwsWSConfigs
	clusterName    string
	clusterType    string
	subClusterType string
	clusterInfo    *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateWorkstation) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return err
	}
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Cluster\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=workstation\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := local.Execute(ctx, command, false)
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

	command = fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --associate-public-ip-address --key-name %s --security-group-ids %s --subnet-id %s --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Cluster,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=workstation}]\"", c.awsWSConfigs.ImageId, c.awsWSConfigs.InstanceType, c.awsWSConfigs.KeyName, c.clusterInfo.publicSecurityGroupId, c.clusterInfo.publicSubnet, c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("run-instances", command))
	stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}

/******************************************************************************/

type DestroyEC struct {
	user           string
	host           string
	clusterName    string
	clusterType    string
	subClusterType string
}

// Execute implements the Task interface
func (c *DestroyEC) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := local.Execute(ctx, command, false)
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
			stdout, _, err = local.Execute(ctx, command, false)
			if err != nil {
				return err
			}
		}
	}

	for idx := 0; idx < 50; idx++ {

		time.Sleep(5 * time.Second)
		command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType, c.subClusterType)
		zap.L().Debug("Command", zap.String("describe-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
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
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
