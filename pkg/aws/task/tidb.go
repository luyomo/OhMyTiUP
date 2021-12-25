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
	"github.com/luyomo/tisample/pkg/ctxt"
	//"github.com/luyomo/tisample/pkg/executor"
	"strings"
	//	"strings"
	"go.uber.org/zap"
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

func getEC2Instances(executor ctxt.Executor, ctx context.Context, clusterName, clusterType, subClusterType string, ptrInstance *[]EC2) error {
	fmt.Printf(" *** *** *** Running inside the create sql server \n\n\n")
	command := fmt.Sprintf("aws ec2 describe-instances --filters \"Name=tag:Name,Values=%s\" \"Name=tag:Cluster,Values=%s\" \"Name=tag:Type,Values=%s\" \"Name=tag:Component,Values=sqlserver\" \"Name=instance-state-code,Values=0,16,32,64,80\"", clusterName, clusterType, subClusterType)
	zap.L().Debug("Command", zap.String("describe-instances", command))
	stdout, _, err := executor.Execute(ctx, command, false)
	if err != nil {
		return err
	}

	var reservations Reservations
	if err = json.Unmarshal(stdout, &reservations); err != nil {
		zap.L().Debug("Json unmarshal", zap.String("describe-instances", string(stdout)))
		return err
	}
	for _, reservation := range reservations.Reservations {
		for _, instance := range reservation.Instances {
			*ptrInstance = append(*ptrInstance, instance)
		}
	}
	return nil
}
