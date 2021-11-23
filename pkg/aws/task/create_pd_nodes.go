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
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"go.uber.org/zap"
	"strings"
)

type ECState struct {
	Code int    `json:"Code"`
	Name string `json:"Name"`
}

type EC2 struct {
	InstanceId       string              `json:"InstanceId"`
	State            ECState             `json:"State"`
	SubnetId         string              `json:"SubnetId"`
	VpcId            string              `json:"VpcId"`
	InstanceType     string              `json:"InstanceType"`
	ImageId          string              `json:"ImageId"`
	PrivateIpAddress string              `json:"PrivateIpAddress"`
	PrivateDnsName   string              `json:"PrivateDnsName"`
	PublicIpAddress  string              `json:"PublicIpAddress"`
	Tags             []map[string]string `json:"Tags"`
}
type NewEC2 struct {
	Instances EC2 `json:"Instances"`
}

type Reservations struct {
	Reservations []EC2s `json:"Reservations"`
}

type EC2s struct {
	Instances []EC2 `json:"Instances"`
}

func (e ECState) String() string {
	return fmt.Sprintf("Code: %s, Name:%s", e.Code, e.Name)
}

func (e EC2) String() string {
	var res []string
	for key, value := range e.Tags {
		res = append(res, fmt.Sprintf("%s->%s", key, value))
	}
	return fmt.Sprintf("InstanceId:%s ,State:%s , SubnetId: %s, VpcId: %s, InstanceType: %s, ImageId: %s, PrivateIpAddress: %s, PrivateDnsName: %s, PublicIpAddress: %s, Tags: <%s>", e.InstanceId, e.State.String(), e.SubnetId, e.VpcId, e.InstanceType, e.ImageId, e.PrivateIpAddress, e.PrivateDnsName, e.PublicIpAddress, strings.Join(res, ","))
}

func (e NewEC2) String() string {
	return e.Instances.String()
}

func (e EC2s) String() string {
	var res []string
	for _, ec2 := range e.Instances {
		res = append(res, ec2.String())
	}
	return fmt.Sprintf(strings.Join(res, ","))
}
func (e Reservations) String() string {
	var res []string
	for _, reservation := range e.Reservations {
		res = append(res, reservation.String())
	}
	return fmt.Sprintf(strings.Join(res, ","))
}

type CreatePDNodes struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
}

// Execute implements the Task interface
func (c *CreatePDNodes) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	if c.awsTopoConfigs.PD.Count == 0 {
		zap.L().Debug("There is no PD nodes to be configured")
		return nil
	}

	// 3. Fetch the count of instance from the instance
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Component\" \"Name=tag-value,Values=pd\" \"Name=instance-state-code,Values=0,16,32,64,80\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return nil
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
		command := fmt.Sprintf("aws ec2 run-instances --count 1 --image-id %s --instance-type %s --key-name %s --security-group-ids %s --subnet-id %s --region %s  --tag-specifications \"ResourceType=instance,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s},{Key=Component,Value=pd}]\"", c.awsTopoConfigs.General.ImageId, c.awsTopoConfigs.PD.InstanceType, c.awsTopoConfigs.General.KeyName, clusterInfo.privateSecurityGroupId, clusterInfo.privateSubnets[_idx%len(clusterInfo.privateSubnets)], c.awsTopoConfigs.General.Region, c.clusterName, c.clusterType)
		zap.L().Debug("Command", zap.String("run-instances", command))
		stdout, _, err = local.Execute(ctx, command, false)
		if err != nil {
			return nil
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
