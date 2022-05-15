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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	//	rdstype "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"io/ioutil"
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
		notExistErr := fmt.Sprintf("Stack with id %s does not exist", clusterName)
		if !strings.Contains(err.Error(), notExistErr) {
			return err
		}
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
		// fmt.Printf("The stacks info here is <%#v> \n\n\n\n", *(stackSummary.StackName))
		// fmt.Printf("The stacks info here is <%#v> \n\n\n\n", stackSummary.StackStatus)
		//		fmt.Printf("The stacks info here is <%#v> \n\n\n\n", stackSummary)

	}

	// input := &cloudformation.DescribeStacksInput{StackName: aws.String(clusterName)}

	// stackInfo, err := client.DescribeStacks(context.TODO(), input)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("The data is <%#v> \n", stackInfo)

	// if len((*stackInfo).Stacks) > 0 {
	// 	return nil
	// }

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

	result, err := client.CreateStack(context.TODO(), stackInput)
	if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return err
	}
	fmt.Printf("The data is <%#v> \n", result)
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

	result, err := client.DeleteStack(context.TODO(), input)
	if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return err
	}
	fmt.Printf("The data is <%#v> \n", result)
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
	//	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := cloudformation.NewFromConfig(cfg)

	input := &cloudformation.DescribeStackResourceInput{StackName: aws.String(clusterName), LogicalResourceId: aws.String("rdsOracleDB")}

	stackResourceInfo, err := client.DescribeStackResource(context.TODO(), input)
	if err != nil {
		notExistErr := fmt.Sprintf("Stack '%s' does not exist", clusterName)
		if strings.Contains(err.Error(), notExistErr) {
			return nil
		}
		return err
	}

	rdsclient := rds.NewFromConfig(cfg)

	rdsDescribeInput := &rds.DescribeDBInstancesInput{DBInstanceIdentifier: aws.String(*(stackResourceInfo.StackResourceDetail.PhysicalResourceId))}
	rdsResourceInfo, err := rdsclient.DescribeDBInstances(context.TODO(), rdsDescribeInput)
	if err != nil {

		return err
	}

	if len(rdsResourceInfo.DBInstances) == 0 {
		return nil
	}

	*(c.tableOracle) = append(*(c.tableOracle), []string{
		*(stackResourceInfo.StackResourceDetail.PhysicalResourceId),
		*(rdsResourceInfo.DBInstances[0].Endpoint.Address),
		strconv.FormatInt(int64(rdsResourceInfo.DBInstances[0].Endpoint.Port), 10),
		*(rdsResourceInfo.DBInstances[0].MasterUsername),
		strconv.FormatInt(int64(rdsResourceInfo.DBInstances[0].AllocatedStorage), 10),
		*(rdsResourceInfo.DBInstances[0].Engine),
		*(rdsResourceInfo.DBInstances[0].EngineVersion),
		*(rdsResourceInfo.DBInstances[0].DBInstanceClass),
		//		*(rdsResourceInfo.DBInstances[0].DBSubnetGroup.DBSubnetGroupName),
		*(rdsResourceInfo.DBInstances[0].VpcSecurityGroups[0].VpcSecurityGroupId),
	})

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
