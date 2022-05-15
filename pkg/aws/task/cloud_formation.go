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
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/session"
//	"github.com/aws/aws-sdk-go/service/cloudformation"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/cloudformation"
    "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/ctxt"
	"io/ioutil"
)

type CreateCloudFormation struct {
	pexecutor                *ctxt.Executor
	awsCloudFormationConfigs *spec.AwsCloudFormationConfigs
	cloudFormationType       string
	clusterInfo              *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateCloudFormation) Execute(ctx context.Context) error {
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        return err
    }
    fmt.Printf("The config is <%#v> \n", cfg)

    client := cloudformation.NewFromConfig(cfg)

    content, _ := ioutil.ReadFile("embed/templates/cloudformation/rds-oracle.yaml")
	templateBody := string(content)
    fmt.Printf("The contents is <%s> \n", templateBody)

	input := &cloudformation.CreateStackInput{
        StackName: aws.String("testjay"),
        TemplateBody: aws.String(templateBody),
        Parameters: []types.Parameter{
            {
                ParameterKey:   aws.String("Username"),
                ParameterValue: aws.String("admin"),
            },
            {
                ParameterKey:   aws.String("DBInsName"),
                ParameterValue: aws.String("jaytest"),
            },
            {
                ParameterKey:   aws.String("Password"),
                ParameterValue: aws.String("1234Abcd"),
            },
        },
	}

    result, err := client.CreateStack(context.TODO(), input)
    if err != nil {
		fmt.Println("Got an error creating an instance:")
		fmt.Println(err)
		return err
	}
    fmt.Printf("The data is <%#v> \n", result)
    return nil

//	stackName := ctx.Value("clusterName").(string)
//	var content []byte
//	if c.awsCloudFormationConfigs.TemplateBodyFilePath != "" {
//		content, _ = ioutil.ReadFile(c.awsCloudFormationConfigs.TemplateBodyFilePath)
//	}
//	templateBody := string(content)
//	var parameters []*cloudformation.Parameter
//	for paramKey, paramValue := range c.awsCloudFormationConfigs.Parameters {
//		parameter := &cloudformation.Parameter{
//			ParameterKey:   aws.String(paramKey),
//			ParameterValue: aws.String(paramValue),
//		}
//		parameters = append(parameters, parameter)
//	}
//
//	sess := session.Must(session.NewSession(
//		&aws.Config{
//			//			Region:     aws.String(c.clusterInfo.region),
//			Region:     aws.String("ap-northeast-1"),
//			MaxRetries: aws.Int(3),
//		}))
//	svc := cloudformation.New(sess)
//	_, err := svc.CreateStack(&cloudformation.CreateStackInput{
//		Parameters:   parameters,
//		StackName:    aws.String(stackName),
//		TemplateBody: aws.String(templateBody),
//		//		TemplateURL:  aws.String(c.awsCloudFormationConfigs.TemplateURL),
//	})
//	// Check stack status
//	// aws cloudformation describe-stacks --stack-name hackathon
//	// CREATE_IN_PROGRESS
//	return err
}

// Rollback implements the Task interface
func (c *CreateCloudFormation) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateCloudFormation) String() string {
	return fmt.Sprintf("Echo: Create CloudFormation ")
}

/******************************************************************************/

type DestroyCloudFormation struct {
	pexecutor   *ctxt.Executor
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyCloudFormation) Execute(ctx context.Context) error {
    return nil
//	stackName := ctx.Value("clusterName").(string)
//	sess := session.Must(session.NewSession(
//		&aws.Config{
//			//			Region:     aws.String(c.clusterInfo.region),
//			Region:     aws.String("ap-northeast-1"),
//			MaxRetries: aws.Int(3),
//		}))
//	svc := cloudformation.New(sess)
//	_, err := svc.DeleteStack(&cloudformation.DeleteStackInput{
//		StackName: aws.String(stackName),
//	})
//	return err
}

// Rollback implements the Task interface
func (c *DestroyCloudFormation) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyCloudFormation) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}
