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
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

func (b *Builder) CreateAurora(awsAuroraConfigs *spec.AwsAuroraConfigs) *Builder {
	var parameters []types.Parameter
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("Username"), ParameterValue: aws.String(awsAuroraConfigs.DBUserName)})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("PubliclyAccessibleFlag"), ParameterValue: aws.String(strconv.FormatBool(awsAuroraConfigs.PubliclyAccessibleFlag))})
	parameters = append(parameters, types.Parameter{ParameterKey: aws.String("Password"), ParameterValue: aws.String(awsAuroraConfigs.DBPassword)})
	if awsAuroraConfigs.CIDR != "" {
		parameters = append(parameters, types.Parameter{ParameterKey: aws.String("VpcCidr"), ParameterValue: aws.String(awsAuroraConfigs.CIDR)})
	}

	if awsAuroraConfigs.DBParameterFamilyGroup != "" {
		parameters = append(parameters, types.Parameter{ParameterKey: aws.String("AuroraFamily"), ParameterValue: aws.String(awsAuroraConfigs.DBParameterFamilyGroup)})
	}

	if awsAuroraConfigs.Engine != "" {
		parameters = append(parameters, types.Parameter{ParameterKey: aws.String("Engine"), ParameterValue: aws.String(awsAuroraConfigs.Engine)})
	}

	if awsAuroraConfigs.EngineVersion != "" {
		parameters = append(parameters, types.Parameter{ParameterKey: aws.String("EngineVersion"), ParameterValue: aws.String(awsAuroraConfigs.EngineVersion)})
	}

	if awsAuroraConfigs.InstanceType != "" {
		parameters = append(parameters, types.Parameter{ParameterKey: aws.String("InstanceType"), ParameterValue: aws.String(awsAuroraConfigs.InstanceType)})
	}

	return b.CreateCloudFormation("embed/templates/cloudformation/aurora.yaml", &parameters, &[]types.Tag{
		{Key: aws.String("Type"), Value: aws.String("aurora")},
		{Key: aws.String("Scope"), Value: aws.String("private")},
	})
}

func (b *Builder) DestroyAurora() *Builder {
	b.tasks = append(b.tasks, &DestroyCloudFormationV2{ /* BaseCloudFormation: BaseCloudFormation{BaseTask: BaseTask{pexecutor: pexecutor}}*/ })
	return b
}

func (b *Builder) CreateCloudFormation(templateFile string, parameters *[]types.Parameter, tags *[]types.Tag) *Builder {
	b.tasks = append(b.tasks, &CreateCloudFormationV2{
		templateFile: templateFile,
		parameters:   parameters,
		tags:         tags,
	})
	return b
}

func (b *Builder) CreateCloudFormationByS3URL(templateURL string, parameters *[]types.Parameter, tags *[]types.Tag) *Builder {
	b.tasks = append(b.tasks, &CreateCloudFormationV2{
		templateURL: templateURL,
		parameters:  parameters,
		tags:        tags,
	})
	return b
}

// func (b *Builder) ListCloudFormationV2(pexecutor *ctxt.Executor, transitGateway *TransitGateway) *Builder {
// 	b.tasks = append(b.tasks, &ListTransitGateway{BaseTransitGateway: BaseTransitGateway{BaseTask: BaseTask{pexecutor: pexecutor}}, transitGateway: transitGateway})
// 	return b
// }

// func (b *Builder) DestroyTransitGatewayV2(pexecutor *ctxt.Executor) *Builder {
// 	b.tasks = append(b.tasks, &DestroyTransitGateway{BaseTransitGateway: BaseTransitGateway{BaseTask: BaseTask{pexecutor: pexecutor}}})
// 	return b
// }

/* **************************************************************************** */
type CloudFormationStatus_Process types.StackStatus

func (p CloudFormationStatus_Process) isState(mode ReadResourceMode) bool {
	switch mode {
	case ReadResourceModeCommon:
		return p.isOKState()
	case ReadResourceModeBeforeCreate:
		return p.isBeforeCreateState()
	case ReadResourceModeAfterCreate:
		return p.isAfterCreateState()
	case ReadResourceModeBeforeDestroy:
		return p.isBeforeDestroyState()
	case ReadResourceModeAfterDestroy:
		return p.isAfterDestroyState()
	}
	return true
}

func (p CloudFormationStatus_Process) isBeforeCreateState() bool {
	return ListContainElement([]string{
		string(types.StackStatusCreateComplete),
		string(types.StackStatusCreateInProgress),
		string(types.StackStatusUpdateInProgress),
		string(types.StackStatusUpdateComplete),
	}, string(p))

}

func (p CloudFormationStatus_Process) isAfterCreateState() bool {
	return ListContainElement([]string{
		string(types.StackStatusCreateComplete),
	}, string(p))

}

func (p CloudFormationStatus_Process) isBeforeDestroyState() bool {
	return p.isBeforeCreateState()
}

func (p CloudFormationStatus_Process) isAfterDestroyState() bool {
	return ListContainElement([]string{
		string(types.StackStatusCreateComplete),
		string(types.StackStatusCreateInProgress),
		string(types.StackStatusUpdateInProgress),
		string(types.StackStatusUpdateComplete),
		string(types.StackStatusDeleteInProgress),
	}, string(p))
}

func (p CloudFormationStatus_Process) isOKState() bool {
	return p.isBeforeCreateState()
}

/******************************************************************************/

type CloudFormationInfo struct {
	BaseResourceInfo
}

func (d *CloudFormationInfo) ToPrintTable() *[][]string {
	tableTransitGateway := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(TransitGateway)
		// tableTransitGateway = append(tableTransitGateway, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableTransitGateway
}

func (d *CloudFormationInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.StackSummary).StackName, nil
	})
}

/******************************************************************************/
type BaseCloudFormation struct {
	BaseTask

	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *cloudformation.Client // Replace the example to specific service
}

func (b *BaseCloudFormation) init(ctx context.Context, mode ReadResourceMode) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = cloudformation.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &CloudFormationInfo{}
	}

	if err := b.readResources(mode); err != nil {
		return err
	}

	return nil
}

func (b *BaseCloudFormation) readResources(mode ReadResourceMode) error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	resp, err := b.client.ListStacks(context.TODO(), &cloudformation.ListStacksInput{})
	if err != nil {
		return err
	}

	for _, stackSummary := range resp.StackSummaries {
		_state := CloudFormationStatus_Process(stackSummary.StackStatus)
		if _state.isState(mode) == true && *stackSummary.StackName == b.clusterName {
			b.ResourceData.Append(stackSummary)
		}
	}
	return nil
}

func (b *BaseTransitGateway) GetStackname() (*string, error) {
	resourceExistFlag, err := b.ResourceData.ResourceExist()
	if err != nil {
		return nil, err
	}

	if resourceExistFlag == false {
		return nil, errors.New("No stack found")
	}

	_data := b.ResourceData.GetData()

	return _data[0].(types.StackSummary).StackName, nil

}

/******************************************************************************/
type CreateCloudFormationV2 struct {
	BaseCloudFormation

	templateFile string
	templateURL  string
	parameters   *[]types.Parameter
	tags         *[]types.Tag
}

// Execute implements the Task interface
func (c *CreateCloudFormationV2) Execute(ctx context.Context) error {
	if err := c.init(ctx, ReadResourceModeAfterCreate); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		*c.parameters = append(*c.parameters, types.Parameter{ParameterKey: aws.String("ClusterName"), ParameterValue: aws.String(c.clusterName)})

		*c.tags = append(*c.tags, types.Tag{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)})
		*c.tags = append(*c.tags, types.Tag{Key: aws.String("Name"), Value: aws.String(c.clusterName)})

		if c.templateURL != "" {
			stackInput := &cloudformation.CreateStackInput{
				StackName:   aws.String(c.clusterName),
				TemplateURL: aws.String(c.templateURL),
				Parameters:  *c.parameters,
				Tags:        *c.tags,
			}

			if _, err = c.client.CreateStack(context.TODO(), stackInput); err != nil {
				return err
			}
		} else {
			content, err := ioutil.ReadFile(c.templateFile)
			if err != nil {
				return err
			}
			templateBody := string(content)

			stackInput := &cloudformation.CreateStackInput{
				StackName:    aws.String(c.clusterName),
				TemplateBody: aws.String(templateBody),
				Parameters:   *c.parameters,
				Tags:         *c.tags,
			}

			if _, err = c.client.CreateStack(context.TODO(), stackInput); err != nil {
				return err
			}
		}

		if err := c.waitUntilResouceAvailable(0, 0, 1, func() error {
			return c.readResources(ReadResourceModeAfterCreate)
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateCloudFormationV2) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateCloudFormationV2) String() string {
	return fmt.Sprintf("Echo: Create CloudFormation ... ...  ")
}

type DestroyCloudFormationV2 struct {
	BaseCloudFormation
}

// Execute implements the Task interface
func (c *DestroyCloudFormationV2) Execute(ctx context.Context) error {
	c.init(ctx, ReadResourceModeBeforeDestroy) // ClusterName/ClusterType and client initialization

	_data := c.ResourceData.GetData()
	for _, stackSummary := range _data {
		_entry := stackSummary.(types.StackSummary)

		if _, err := c.client.DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{StackName: _entry.StackName}); err != nil {
			return err
		}

	}

	if err := c.waitUntilResouceDestroy(0, 0, func() error {
		return c.readResources(ReadResourceModeAfterDestroy)
	}); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyCloudFormationV2) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyCloudFormationV2) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}
