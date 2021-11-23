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

// Mkdir is used to create directory on the target host
type CreateInternetGateway struct {
	user           string
	host           string
	awsTopoConfigs *spec.AwsTopoConfigs
	clusterName    string
	clusterType    string
}

// Execute implements the Task interface
func (c *CreateInternetGateway) Execute(ctx context.Context) error {
	local, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: c.user})
	if err != nil {
		return nil
	}

	command := fmt.Sprintf("aws ec2 describe-internet-gateways --filters \"Name=tag-key,Values=Name\" \"Name=tag-value,Values=%s\" \"Name=tag-key,Values=Type\" \"Name=tag-value,Values=%s\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("describe-internet-gateways", command))
	stdout, _, err := local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	var internetGateways InternetGateways
	if err = json.Unmarshal(stdout, &internetGateways); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("subnets", string(stdout)))
		return nil
	}

	if len(internetGateways.InternetGateways) > 0 {
		zap.L().Info("Internetgateways existed", zap.String("Internetgateway", internetGateways.String()))
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-internet-gateway --tag-specifications \"ResourceType=internet-gateway,Tags=[{Key=Name,Value=%s},{Key=Type,Value=%s}]\"", c.clusterName, c.clusterType)
	zap.L().Debug("Command", zap.String("create-internet-gateway", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}
	var newInternetGateway NewInternetGateway
	if err = json.Unmarshal(stdout, &newInternetGateway); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("new internet gateway", string(stdout)))
		return nil
	}

	zap.L().Debug("New Internet gateway", zap.String("newInternetGateway", newInternetGateway.String()))
	//	fmt.Printf("The stdout from the internet gateway preparation: %#v \n\n\n", newInternetGateway)
	command = fmt.Sprintf("aws ec2 attach-internet-gateway --internet-gateway-id %s --vpc-id %s", newInternetGateway.InternetGateway.InternetGatewayId, clusterInfo.vpcInfo.VpcId)
	zap.L().Debug("Command", zap.String("create-internet-gateway", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	command = fmt.Sprintf("aws ec2 create-route --route-table-id %s --destination-cidr-block 0.0.0.0/0 --gateway-id %s", clusterInfo.publicRouteTableId, newInternetGateway.InternetGateway.InternetGatewayId)
	zap.L().Debug("Command", zap.String("create-route", command))
	stdout, _, err = local.Execute(ctx, command, false)
	if err != nil {
		return nil
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateInternetGateway) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateInternetGateway) String() string {
	return fmt.Sprintf("Echo: host=%s ", c.host)
}
