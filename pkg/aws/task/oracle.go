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
	"fmt"
	"strconv"

	"io/ioutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type CreateOracle struct {
	pexecutor        *ctxt.Executor
	awsOracleConfigs *spec.AwsOracleConfigs
	clusterInfo      *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateOracle) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}

	client := cloudformation.NewFromConfig(cfg)

	listStacksInput := &cloudformation.ListStacksInput{}
	listStacks, err := client.ListStacks(context.TODO(), listStacksInput)
	if err != nil {
		return err
	}
	for _, stackSummary := range listStacks.StackSummaries {
		if *(stackSummary.StackName) == clusterName && stackSummary.StackStatus != "DELETE_COMPLETE" {
			return nil
		}
	}

	content, _ := ioutil.ReadFile("embed/templates/cloudformation/rds-oracle.yaml")
	templateBody := string(content)

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{
		ParameterKey:   aws.String("Username"),
		ParameterValue: aws.String(c.awsOracleConfigs.DBUserName),
	})

	parameters = append(parameters, types.Parameter{
		ParameterKey:   aws.String("DBInsName"),
		ParameterValue: aws.String(c.awsOracleConfigs.DBInstanceName),
	})

	parameters = append(parameters, types.Parameter{
		ParameterKey:   aws.String("Password"),
		ParameterValue: aws.String(c.awsOracleConfigs.DBPassword),
	})

	if c.awsOracleConfigs.CIDR != "" {
		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("VpcCidr"),
			ParameterValue: aws.String(c.awsOracleConfigs.CIDR),
		})
	}

	if c.awsOracleConfigs.InstanceType != "" {
		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("InstanceType"),
			ParameterValue: aws.String(c.awsOracleConfigs.InstanceType),
		})
	}

	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(clusterName),
		TemplateBody: aws.String(templateBody),
		Parameters:   parameters,
		Tags: []types.Tag{
			{
				Key:   aws.String("Cluster"),
				Value: aws.String(clusterType),
			},
			{
				Key:   aws.String("Type"),
				Value: aws.String("oracle"),
			},
			{
				Key:   aws.String("Name"),
				Value: aws.String(clusterName),
			},
		},
	}

	_, err = client.CreateStack(context.TODO(), stackInput)
	if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (c *CreateOracle) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateOracle) String() string {
	return fmt.Sprintf("Echo: Create Oracle by cloud formation template ")
}

/******************************************************************************/

type DestroyOracle struct {
	pexecutor   *ctxt.Executor
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyOracle) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := cloudformation.NewFromConfig(cfg)

	input := &cloudformation.DeleteStackInput{
		StackName: aws.String(clusterName),
	}

	_, err = client.DeleteStack(context.TODO(), input)
	if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (c *DestroyOracle) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyOracle) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}

// ----- Oracle
type ListOracle struct {
	pexecutor   *ctxt.Executor
	tableOracle *[][]string
}

// Execute implements the Task interface
func (c *ListOracle) Execute(ctx context.Context) error {
	fmt.Printf("Listing the resources for oracle.   \n\n\n\n")
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	oracleInstanceInfos, err := utils.ExtractInstanceRDSInfo(clusterName, clusterType, "oracle")
	if err != nil {
		return err
	}

	for _, oraInsInfo := range *oracleInstanceInfos {
		*(c.tableOracle) = append(*(c.tableOracle), []string{
			oraInsInfo.PhysicalResourceId,
			oraInsInfo.DBName,
			oraInsInfo.EndPointAddress,
			strconv.FormatInt(oraInsInfo.DBPort, 10),
			oraInsInfo.DBUserName,
			strconv.FormatInt(oraInsInfo.DBSize, 10),
			oraInsInfo.DBEngine,
			oraInsInfo.DBEngineVersion,
			oraInsInfo.DBInstanceClass,
			oraInsInfo.VpcSecurityGroupId,
		})
	}
	return nil
}

// Rollback implements the Task interface
func (c *ListOracle) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListOracle) String() string {
	return fmt.Sprintf("Echo: List Oracle ")
}
