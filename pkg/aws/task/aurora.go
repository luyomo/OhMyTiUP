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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/utils"
	"github.com/luyomo/tisample/pkg/ctxt"
	"io/ioutil"
)

type CreateAurora struct {
	pexecutor        *ctxt.Executor
	awsAuroraConfigs *spec.AwsAuroraConfigs
	clusterInfo      *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateAurora) Execute(ctx context.Context) error {
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

	content, _ := ioutil.ReadFile("embed/templates/cloudformation/aurora.yaml")
	templateBody := string(content)

	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{
		ParameterKey:   aws.String("Username"),
		ParameterValue: aws.String(c.awsAuroraConfigs.DBUserName),
	})

	parameters = append(parameters, types.Parameter{
		ParameterKey:   aws.String("PubliclyAccessibleFlag"),
		ParameterValue: aws.String(strconv.FormatBool(c.awsAuroraConfigs.PubliclyAccessibleFlag)),
	})

	parameters = append(parameters, types.Parameter{
		ParameterKey:   aws.String("Password"),
		ParameterValue: aws.String(c.awsAuroraConfigs.DBPassword),
	})

	if c.awsAuroraConfigs.CIDR != "" {
		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("VpcCidr"),
			ParameterValue: aws.String(c.awsAuroraConfigs.CIDR),
		})
	}

	if c.awsAuroraConfigs.InstanceType != "" {
		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("InstanceType"),
			ParameterValue: aws.String(c.awsAuroraConfigs.InstanceType),
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
				Value: aws.String("aurora"),
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

	for cnt := 0; cnt < 60; cnt++ {
		describeStackInput := &cloudformation.DescribeStacksInput{
			StackName: aws.String(clusterName),
		}

		stackInfo, err := client.DescribeStacks(context.TODO(), describeStackInput)
		if err != nil {
			fmt.Println("Got an error creating an instance:")
			fmt.Println(err)
			return err
		}

		if (*stackInfo).Stacks[0].StackStatus == "CREATE_COMPLETE" || (*stackInfo).Stacks[0].StackStatus == "CREATE_FAILED" {
			break
		}

		time.Sleep(60 * time.Second)
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateAurora) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateAurora) String() string {
	return fmt.Sprintf("Echo: Create Aurora by cloud formation template ")
}

/******************************************************************************/

type DestroyAurora struct {
	pexecutor   *ctxt.Executor
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyAurora) Execute(ctx context.Context) error {
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
func (c *DestroyAurora) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyAurora) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}

// ----- Aurora
type ListAurora struct {
	pexecutor   *ctxt.Executor
	tableAurora *[][]string
}

// Execute implements the Task interface
func (c *ListAurora) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	auroraInstanceInfos, err := utils.ExtractInstanceOracleInfo(clusterName, clusterType, "aurora")
	if err != nil {
		return err
	}

	for _, auroraInsInfo := range *auroraInstanceInfos {
		*(c.tableAurora) = append(*(c.tableAurora), []string{
			auroraInsInfo.PhysicalResourceId,
			auroraInsInfo.EndPointAddress,
			strconv.FormatInt(auroraInsInfo.DBPort, 10),
			auroraInsInfo.DBUserName,
			auroraInsInfo.DBEngine,
			auroraInsInfo.DBEngineVersion,
			auroraInsInfo.DBInstanceClass,
			auroraInsInfo.VpcSecurityGroupId,
		})
	}
	return nil
}

// Rollback implements the Task interface
func (c *ListAurora) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListAurora) String() string {
	return fmt.Sprintf("Echo: List Aurora ")
}
