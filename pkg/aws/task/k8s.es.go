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
	// "encoding/json"
	"errors"
	"fmt"
	// "time"

	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	// iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

type DeployK8SES struct {
	pexecutor         *ctxt.Executor
	awsWSConfigs      *spec.AwsWSConfigs
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
}

// Execute implements the Task interface
func (c *DeployK8SES) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	// tagProject := GetProject(ctx)
	// tagOwner, _, err := GetCallerUser(ctx)
	// if err != nil {
	// 	return err
	// }

	/* ********** ********** 001. Prepare execution context  **********/
	// 001.01. Get all the workstation nodes
	workstation, err := GetWSExecutor02(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile, true, nil)
	if err != nil {
		return err
	}

	// describeNodegroupInput := &eks.DescribeNodegroupInput{ClusterName: aws.String(clusterName), NodegroupName: aws.String("esNodeGroup")}
	// describeNodegroup, err := clientEks.DescribeNodegroup(context.TODO(), describeNodegroupInput)
	// if err != nil {
	// 	return err
	// }

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	// client := ec2.NewFromConfig(cfg)
	/***************************************************************************************************************
	 * 02. Check template existness
	 * Search template name
	 *     01. go to next step if the template has been created.
	 *     02. Create the template if it does not exist
	 ***************************************************************************************************************/

	clientEks := eks.NewFromConfig(cfg)

	clientIam := iam.NewFromConfig(cfg)
	var roleArn string
	getRoleInput := &iam.GetRoleInput{RoleName: aws.String(clusterName)}

	getRoleOutput, err := clientIam.GetRole(context.TODO(), getRoleInput)
	if err != nil {
		return err
	}
	roleArn = *getRoleOutput.Role.Arn

	// combinedName := fmt.Sprintf("%s.%s.%s.%s", clusterType, clusterName, c.subClusterType, "eks")
	// describeLaunchTemplatesInput := &ec2.DescribeLaunchTemplatesInput{LaunchTemplateNames: []string{combinedName}}
	// if _, err := client.DescribeLaunchTemplates(context.TODO(), describeLaunchTemplatesInput); err != nil {
	// 	fmt.Printf("Calling the launch template inout ... ... ... <%#v>  \n\n\n\n", err.Error())
	// 	var ae smithy.APIError
	// 	if errors.As(err, &ae) {
	// 		fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
	// 		if ae.ErrorCode() == "InvalidLaunchTemplateName.NotFoundException" {
	// 			fmt.Printf("--------------------------- \n\n\n")
	// 			c.CreateLaunchTemplate(client, &combinedName, clusterName, clusterType, tagOwner, tagProject)
	// 		} else {
	// 			return err
	// 		}
	// 	} else {
	// 		return err
	// 	}
	// }

	var parallelTasks []Task
	parallelTasks = append(parallelTasks, &DeployEKSNodeGroup{
		pexecutor:         c.pexecutor,
		awsGeneralConfigs: c.awsGeneralConfigs,
		subClusterType:    c.subClusterType,
		clusterInfo:       c.clusterInfo,
		nodeGroupName:     "test001",
	})

	parallelTasks = append(parallelTasks, &DeployEKSNodeGroup{
		pexecutor:         c.pexecutor,
		awsGeneralConfigs: c.awsGeneralConfigs,
		subClusterType:    c.subClusterType,
		clusterInfo:       c.clusterInfo,
		nodeGroupName:     "test002",
	})

	parallelExe := Parallel{ignoreError: false, inner: parallelTasks}
	if err := parallelExe.Execute(ctx); err != nil {
		return err
	}

	err = (*workstation).TransferTemplate(ctx, "templates/config/eks/es.ingress.yaml.tpl", "/opt/helm/es.ingress.yaml", "0600", nil, true, 0)
	if err != nil {
		return err
	}

	err = (*workstation).TransferTemplate(ctx, "templates/config/eks/es.values.yaml.tpl", "/opt/helm/es.values.yaml", "0600", nil, true, 0)
	if err != nil {
		return err
	}

	_cmds := []string{
		"helm repo add elastic https://helm.elastic.co",
		"helm install elasticsearch elastic/elasticsearch -f /opt/helm/es.values.yaml",
		"kubectl create -f /opt/helm/es.ingress.yaml",
	}
	for _, _cmd := range _cmds {
		if _, _, err = (*workstation).Execute(ctx, _cmd, false); err != nil {
			return err
		}
	}

	return nil

	listNodegroupsInput := &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)}
	listNodegroup, err := clientEks.ListNodegroups(context.TODO(), listNodegroupsInput)
	if err != nil {
		return err
	}

	if containString(listNodegroup.Nodegroups, "esNodeGroup") == false {

		// Node group creation
		nodegroupScalingConfig := &types.NodegroupScalingConfig{DesiredSize: aws.Int32(1), MaxSize: aws.Int32(1), MinSize: aws.Int32(1)}
		createNodegroupInput := &eks.CreateNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodeRole:      aws.String(roleArn),
			NodegroupName: aws.String("esNodeGroup"),
			Subnets:       c.clusterInfo.privateSubnets,
			InstanceTypes: []string{"c5.xlarge"},
			DiskSize:      aws.Int32(20),
			ScalingConfig: nodegroupScalingConfig}
		// -- Below command is used for customized IAM. But at the same time, the iam has to be build for eks.
		// --https://aws.amazon.com/blogs/containers/introducing-launch-template-and-custom-ami-support-in-amazon-eks-managed-node-groups/
		// createNodegroupInput := &eks.CreateNodegroupInput{
		// 	ClusterName:   aws.String(clusterName),
		// 	NodeRole:      aws.String(roleArn),
		// 	NodegroupName: aws.String("esNodeGroup"),
		// 	Subnets:       c.clusterInfo.privateSubnets,
		// 	ScalingConfig: nodegroupScalingConfig,
		// 	// AmiType:       types.AMITypes(combinedName),
		// 	AmiType: types.AMITypes("CUSTOM"),
		// 	LaunchTemplate: &types.LaunchTemplateSpecification{
		// 		Name:    aws.String(combinedName),
		// 		Version: aws.String("$Latest"),
		// 	}}

		createNodegroup, err := clientEks.CreateNodegroup(context.TODO(), createNodegroupInput)
		if err != nil {
			return err
		}
		fmt.Printf("The create node group is <%#v>\n\n\n", createNodegroup)
	}

	fmt.Printf("-----------------------------------\n\n\n")
	fmt.Printf("The list group is <%#v> \n\n\n\n\n", listNodegroup.Nodegroups)
	if containString(listNodegroup.Nodegroups, "elasticsearch") == false {
		// Node group creation
		labels := map[string]string{"dedicated": "elastic"}
		var taints []types.Taint
		taints = append(taints, types.Taint{Effect: "NO_SCHEDULE", Key: aws.String("dedicated"), Value: aws.String("elastic")})

		nodegroupScalingConfig := &types.NodegroupScalingConfig{DesiredSize: aws.Int32(3), MaxSize: aws.Int32(3), MinSize: aws.Int32(3)}
		createNodegroupInput := &eks.CreateNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodeRole:      aws.String(roleArn),
			NodegroupName: aws.String("elasticsearch"),
			Subnets:       c.clusterInfo.privateSubnets,
			InstanceTypes: []string{"c5.xlarge"},
			DiskSize:      aws.Int32(20),
			ScalingConfig: nodegroupScalingConfig,
			Labels:        labels,
			Taints:        taints}

		createNodegroup, err := clientEks.CreateNodegroup(context.TODO(), createNodegroupInput)
		if err != nil {
			return err
		}
		fmt.Printf("The create node group is <%#v>\n\n\n", createNodegroup)

	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployK8SES) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployK8SES) String() string {
	return fmt.Sprintf("Echo: Deploying EKS Cluster")
}

func (c *DeployK8SES) CreateLaunchTemplate(client *ec2.Client, templateName *string, clusterName, clusterType, tagOwner, tagProject string) error {
	fmt.Printf("Calling inseid the CreateLaunchTemplate \n\n\n\n\n")
	requestLaunchTemplateData := ec2types.RequestLaunchTemplateData{}

	// 02. Storage template preparation
	var launchTemplateBlockDeviceMappingRequest []ec2types.LaunchTemplateBlockDeviceMappingRequest
	rootBlockDeviceMapping := ec2types.LaunchTemplateBlockDeviceMappingRequest{
		DeviceName: aws.String("/dev/xvda"),
		Ebs: &ec2types.LaunchTemplateEbsBlockDeviceRequest{
			DeleteOnTermination: aws.Bool(true),
			VolumeSize:          aws.Int32(8),
			VolumeType:          ec2types.VolumeType("gp2"),
		},
	}

	launchTemplateBlockDeviceMappingRequest = append(launchTemplateBlockDeviceMappingRequest, rootBlockDeviceMapping)
	// if c.awsTopoConfigs.VolumeType != "" {
	// 	blockDeviceMapping := types.LaunchTemplateBlockDeviceMappingRequest{
	// 		DeviceName: aws.String("/dev/sdb"),
	// 		Ebs: &types.LaunchTemplateEbsBlockDeviceRequest{
	// 			DeleteOnTermination: aws.Bool(true),
	// 			Iops:                aws.Int32(int32(c.awsTopoConfigs.Iops)),
	// 			VolumeSize:          aws.Int32(int32(c.awsTopoConfigs.VolumeSize)),
	// 			VolumeType:          types.VolumeType(c.awsTopoConfigs.VolumeType),
	// 		},
	// 	}

	// 	launchTemplateBlockDeviceMappingRequest = append(launchTemplateBlockDeviceMappingRequest, blockDeviceMapping)
	// }
	requestLaunchTemplateData.BlockDeviceMappings = launchTemplateBlockDeviceMappingRequest
	requestLaunchTemplateData.EbsOptimized = aws.Bool(false)                                            // EbsOptimized flag, not support all the instance type
	requestLaunchTemplateData.ImageId = aws.String(c.awsGeneralConfigs.ImageId)                         // ImageID
	requestLaunchTemplateData.InstanceType = ec2types.InstanceType((*c.awsGeneralConfigs).InstanceType) // Instance Type
	requestLaunchTemplateData.KeyName = aws.String(c.awsGeneralConfigs.KeyName)                         // Key name
	requestLaunchTemplateData.SecurityGroupIds = []string{c.clusterInfo.privateSecurityGroupId}         // security group

	tags := []ec2types.Tag{
		{Key: aws.String("Cluster"), Value: aws.String(clusterType)},   // ex: ohmytiup-tidb
		{Key: aws.String("Type"), Value: aws.String(c.subClusterType)}, // ex: tidb/oracle/workstation
		{Key: aws.String("Component"), Value: aws.String("eks")},       // ex: tidb/tikv/pd
		{Key: aws.String("Name"), Value: aws.String(clusterName)},      // ex: clustertest
		{Key: aws.String("Owner"), Value: aws.String(tagOwner)},        // ex: aws-user
		{Key: aws.String("Project"), Value: aws.String(tagProject)},    // ex: clustertest
	}

	// 03. Template data preparation
	fmt.Printf("Creating the template <%s> \n\n\n\n", *templateName)
	var tagSpecification []ec2types.TagSpecification
	tagSpecification = append(tagSpecification, ec2types.TagSpecification{ResourceType: ec2types.ResourceTypeLaunchTemplate, Tags: tags})
	createLaunchTemplateInput := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: templateName,
		LaunchTemplateData: &requestLaunchTemplateData,
		TagSpecifications:  tagSpecification,
	}

	// 04. Template generation
	createLaunchTemplate, err := client.CreateLaunchTemplate(context.TODO(), createLaunchTemplateInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
		}
		return err
	}
	fmt.Printf("The ourtput is <%#v> \n\n\n\n", createLaunchTemplate)

	return nil
}
