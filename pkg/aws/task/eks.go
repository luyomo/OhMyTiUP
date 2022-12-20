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
	"fmt"
	// "time"

	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	// "go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

type DeployEKS struct {
	pexecutor      *ctxt.Executor
	awsWSConfigs   *spec.AwsWSConfigs
	subClusterType string
	clusterInfo    *ClusterInfo
}

// type KafkaNodes struct {
// 	All            []string
// 	Zookeeper      []string
// 	Broker         []string
// 	SchemaRegistry []string
// 	RestService    []string
// 	Connector      []string
// }

// Execute implements the Task interface
func (c *DeployEKS) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	tagProject := GetProject(ctx)
	tagOwner, tagAccountID, err := GetCallerUser(ctx)
	if err != nil {
		return err
	}

	/* ********** ********** 001. Prepare execution context  **********/
	// 001.01. Get all the workstation nodes
	workstation, err := GetWSExecutor02(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile, true, nil)
	if err != nil {
		return err
	}

	// 001.02. Send the access key to workstation
	// err = (*workstation).Transfer(ctx, c.clusterInfo.keyFile, "~/.ssh/id_rsa", false, 0)
	// if err != nil {
	// 	return err
	// }

	// _, _, err = (*workstation).Execute(ctx, `chmod 600 ~/.ssh/id_rsa`, false)
	// if err != nil {
	// 	return err
	// }

	if _, _, err = (*workstation).Execute(ctx, `apt-get update`, true); err != nil {
		return err
	}

	/* ********** ********** 002.Helm setup ********** */
	if _, _, err = (*workstation).Execute(ctx, `curl -L https://git.io/get_helm.sh | bash -s -- --version v3.8.2`, true); err != nil {
		return err
	}

	/* ********** ********** 004. eksctl install ********** */
	if _, _, err = (*workstation).Execute(ctx, `mkdir -p /opt/k8s`, true); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, `curl --silent --location "https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_$(uname -s)_amd64.tar.gz" | tar xz -C /tmp`, true); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, `sudo mv /tmp/eksctl /usr/local/bin`, true); err != nil {
		return err
	}

	/* ********** ********** 003.EKS cluster generation ********** */
	err = (*workstation).TransferTemplate(ctx, "templates/config/nodeGroup.yaml.tpl", "/opt/k8s/eks.yaml", "0644", nil, true, 0)
	if err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	clientIam := iam.NewFromConfig(cfg)

	getRoleInput := &iam.GetRoleInput{RoleName: aws.String(clusterName)}

	getRoleOutput, err := clientIam.GetRole(context.TODO(), getRoleInput)
	if err != nil {
		return err
	}

	var roleArn string
	if getRoleOutput == nil {

		tags := []iamTypes.Tag{
			{Key: aws.String("Cluster"), Value: aws.String(clusterType)},   // ex: ohmytiup-tidb
			{Key: aws.String("Type"), Value: aws.String(c.subClusterType)}, // ex: tidb/oracle/workstation
			{Key: aws.String("Name"), Value: aws.String(clusterName)},      // ex: clustertest
			{Key: aws.String("Owner"), Value: aws.String(tagOwner)},        // ex: aws-user
			{Key: aws.String("Project"), Value: aws.String(tagProject)},    // ex: clustertest
		}

		createRoleInput := &iam.CreateRoleInput{AssumeRolePolicyDocument: aws.String("{\"Version\": \"2012-10-17\",\"Statement\": [{\"Effect\": \"Allow\", \"Principal\": {\"Service\": \"eks.amazonaws.com\"},\"Action\": \"sts:AssumeRole\"}]}"), RoleName: aws.String(clusterName), Tags: tags}

		createRole, err := clientIam.CreateRole(context.TODO(), createRoleInput)
		if err != nil {
			return err
		}
		roleArn = *createRole.Role.Arn
		fmt.Printf("The created policy is <%#v> \n\n\n", createRole)
	} else {
		roleArn = *getRoleOutput.Role.Arn
	}
	fmt.Printf("The role arn is <%s> \n\n\n", roleArn)

	attachRolePolicyInput := &iam.AttachRolePolicyInput{PolicyArn: aws.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"), RoleName: aws.String(clusterName)}

	attachRolePolicy, err := clientIam.AttachRolePolicy(context.TODO(), attachRolePolicyInput)
	if err != nil {
		return err
	}
	fmt.Printf("The attached role policy is <%#v> \n\n\n\n", attachRolePolicy)

	clientEks := eks.NewFromConfig(cfg)
	// 001. VpcConfigRequest

	describeClusterInput := &eks.DescribeClusterInput{Name: aws.String(clusterName)}

	describeCluster, err := clientEks.DescribeCluster(context.TODO(), describeClusterInput)
	if err != nil {
		return err
	}
	fmt.Printf("The found group is <%#v> \n\n\n", describeCluster)
	if describeCluster == nil {
		// fmt.Printf("All the subnets are <%#v> \n\n\n\n", c.clusterInfo)
		vpcConfigRequest := &types.VpcConfigRequest{EndpointPrivateAccess: aws.Bool(true), SubnetIds: c.clusterInfo.privateSubnets}

		createClusterInput := &eks.CreateClusterInput{Name: aws.String(clusterName), ResourcesVpcConfig: vpcConfigRequest, RoleArn: aws.String(roleArn)}

		createCluster, err := clientEks.CreateCluster(context.TODO(), createClusterInput)
		if err != nil {
			return err
		}
		fmt.Printf("The result from create clutster is : <{%#v}>", createCluster)

	}
	fmt.Printf("The cluster info is <%s> \n\n\n\n", *describeCluster.Cluster.Identity.Oidc.Issuer)

	// IAM provider set up: CreateOpenIDConnectProviderInput
	// createOpenIDConnectProviderInput := &iam.CreateOpenIDConnectProviderInput{Url: describeCluster.Cluster.Identity.Oidc.Issuer, ClientIDList: []string{"sts.amazonaws.com"}}
	// createOpenIDConnectProvider, err := clientIam.CreateOpenIDConnectProvider(context.TODO(), createOpenIDConnectProviderInput)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("The open id : <%#v> \n\n\n\n", createOpenIDConnectProvider)

	// oidcRequest := &types.OidcIdentityProviderConfigRequest{ClientId: aws.String("sts.amazonaws.com"), IdentityProviderConfigName: aws.String("EAADCCE3D0AB71B4010FF90AFEFA69A7"), IssuerUrl: describeCluster.Cluster.Identity.Oidc.Issuer}
	// associateIdentityProviderConfigInput := &eks.AssociateIdentityProviderConfigInput{ClusterName: aws.String(clusterName), Oidc: oidcRequest}
	// associateIdentityProviderConfig, err := clientEks.AssociateIdentityProviderConfig(context.TODO(), associateIdentityProviderConfigInput)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("The associate id is <%#v> \n\n\n", associateIdentityProviderConfig)

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`eksctl utils associate-iam-oidc-provider --cluster %s --approve`, clusterName), false); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`eksctl create iamserviceaccount \
   --name ebs-csi-controller-sa \
   --namespace kube-system \
   --cluster %s \
   --attach-policy-arn arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy \
   --approve   --role-only   --role-name AmazonEKS_EBS_CSI_DriverRole`, clusterName), false); err != nil {
		return err
	}

	describeAddonInput := &eks.DescribeAddonInput{AddonName: aws.String("aws-ebs-csi-driver"), ClusterName: aws.String(clusterName)}
	describeAddon, err := clientEks.DescribeAddon(context.TODO(), describeAddonInput)
	if err != nil {
		return err
	}

	if describeAddon == nil {

		createAddonInput := &eks.CreateAddonInput{AddonName: aws.String("aws-ebs-csi-driver"), ClusterName: aws.String(clusterName), ServiceAccountRoleArn: aws.String(fmt.Sprintf("arn:aws:iam::%s:role/AmazonEKS_EBS_CSI_DriverRole", tagAccountID))}
		createAddon, err := clientEks.CreateAddon(context.TODO(), createAddonInput)
		if err != nil {
			return err
		}
		fmt.Printf("Create addon is <%#v> \n\n\n", createAddon)
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployEKS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployEKS) String() string {
	return fmt.Sprintf("Echo: Deploying EKS Cluster")
}
