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
	// "errors"
	"fmt"
	"strings"
	// "time"

	// "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	// "github.com/aws/aws-sdk-go-v2/service/ec2"
	// ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	// iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
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
		// "helm upgrade elasticsearch elastic/elasticsearch -f /opt/helm/es.values.yaml",
		"kubectl apply -f /opt/helm/es.ingress.yaml",
	}
	for _, _cmd := range _cmds {
		if _, _, err = (*workstation).Execute(ctx, _cmd, false); err != nil {
			return err
		}
	}

	esExistFlag, err := HelmResourceExist(workstation, "elasticsearch")
	if err != nil {
		return err
	}

	if esExistFlag == false {
		if _, _, err = (*workstation).Execute(ctx, "helm install elasticsearch elastic/elasticsearch -f /opt/helm/es.values.yaml", false); err != nil {
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

type DestroyK8SES struct {
	pexecutor *ctxt.Executor
	gOpt      operator.Options
	// awsWSConfigs      *spec.AwsWSConfigs
	// awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	// subClusterType    string
	// clusterInfo       *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyK8SES) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}
	clientEks := eks.NewFromConfig(cfg)

	listClustersInput := &eks.ListClustersInput{}
	listClusters, err := clientEks.ListClusters(context.TODO(), listClustersInput)
	if err != nil {
		return err
	}
	fmt.Printf("The data is <%#v> \n\n\n", listClusters.Clusters)
	for _, _cluster := range listClusters.Clusters {
		if _cluster == clusterName {
			workstation, err := GetWSExecutor02(*c.pexecutor, ctx, clusterName, clusterType, c.gOpt.SSHUser, c.gOpt.IdentityFile, true, nil)
			if err != nil {
				return err
			}

			fmt.Printf("Starting to remove the cluster \n\n\n")

			esExistFlag, err := HelmResourceExist(workstation, "elasticsearch")
			if err != nil {
				return err
			}

			if esExistFlag == true {
				if _, _, err = (*workstation).Execute(ctx, "helm delete elasticsearch", false); err != nil {
					return err
				}
			}

			stdout, _, err := (*workstation).Execute(ctx, "kubectl get pvc --selector='app=elasticsearch-master' -o jsonpath=\"{.items[*]['metadata.name']}\"", false)
			if err != nil {
				return err
			}
			pvcList := strings.Split(string(stdout), " ")

			for _, _pvc := range pvcList {
				if _pvc == "" {
					continue
				}
				if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("kubectl delete pvc %s", _pvc), false); err != nil {
					return err
				}
			}

			var parallelTasks []Task
			parallelTasks = append(parallelTasks, &DestroyEKSNodeGroup{
				pexecutor:     c.pexecutor,
				nodeGroupName: "test001",
			})

			parallelTasks = append(parallelTasks, &DestroyEKSNodeGroup{
				pexecutor:     c.pexecutor,
				nodeGroupName: "test002",
			})

			parallelExe := Parallel{ignoreError: false, inner: parallelTasks}
			if err := parallelExe.Execute(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyK8SES) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyK8SES) String() string {
	return fmt.Sprintf("Echo: Destroying ES on eks Cluster")
}
