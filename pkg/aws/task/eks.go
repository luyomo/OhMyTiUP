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
	"time"

	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	operator "github.com/luyomo/OhMyTiUP/pkg/aws/operation"
)

type DeployEKS struct {
	pexecutor         *ctxt.Executor
	awsWSConfigs      *spec.AwsWSConfigs
	awsGeneralConfigs *spec.AwsTopoConfigsGeneral
	subClusterType    string
	clusterInfo       *ClusterInfo
}

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

	if _, _, err = (*workstation).Execute(ctx, `apt-get update`, true); err != nil {
		return err
	}

	/* ********** ********** 002.Helm setup ********** */
	if _, _, err = (*workstation).Execute(ctx, `curl -L https://git.io/get_helm.sh | bash -s -- --version v3.8.2`, true); err != nil {
		return err
	}

	/* ********** ********** 003. eksctl install ********** */
	if _, _, err = (*workstation).Execute(ctx, `mkdir -p /opt/k8s`, true); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, `curl --silent --location "https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_$(uname -s)_amd64.tar.gz" | tar xz -C /tmp`, true); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, `mv /tmp/eksctl /usr/local/bin`, true); err != nil {
		return err
	}

	/* ********** ********** 003.EKS cluster generation ********** */
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	clientIam := iam.NewFromConfig(cfg)

	var roleArn string
	getRoleInput := &iam.GetRoleInput{RoleName: aws.String(clusterName)}

	getRoleOutput, err := clientIam.GetRole(context.TODO(), getRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "NoSuchEntity" {
				tags := []iamTypes.Tag{
					{Key: aws.String("Cluster"), Value: aws.String(clusterType)},   // ex: ohmytiup-tidb
					{Key: aws.String("Type"), Value: aws.String(c.subClusterType)}, // ex: tidb/oracle/workstation
					{Key: aws.String("Name"), Value: aws.String(clusterName)},      // ex: clustertest
					{Key: aws.String("Owner"), Value: aws.String(tagOwner)},        // ex: aws-user
					{Key: aws.String("Project"), Value: aws.String(tagProject)},    // ex: clustertest
				}

				createRoleInput := &iam.CreateRoleInput{AssumeRolePolicyDocument: aws.String("{\"Version\": \"2012-10-17\",\"Statement\": [{\"Effect\": \"Allow\", \"Principal\": {\"Service\": \"eks.amazonaws.com\"},\"Action\": \"sts:AssumeRole\"}, {\"Effect\": \"Allow\",\"Principal\": {\"Service\": \"ec2.amazonaws.com\"},\"Action\": \"sts:AssumeRole\"}]}"), RoleName: aws.String(clusterName), Tags: tags}

				createRole, err := clientIam.CreateRole(context.TODO(), createRoleInput)
				if err != nil {
					return err
				}
				roleArn = *createRole.Role.Arn

			} else {
				return err
			}
		} else {
			return err
		}

	} else {
		roleArn = *getRoleOutput.Role.Arn
	}

	for _, policy := range []string{"AmazonEKSClusterPolicy", "AmazonEKSWorkerNodePolicy", "AmazonEC2ContainerRegistryReadOnly", "AmazonEKS_CNI_Policy", "AmazonSSMManagedInstanceCore"} {
		attachRolePolicyInput := &iam.AttachRolePolicyInput{PolicyArn: aws.String(fmt.Sprintf("arn:aws:iam::aws:policy/%s", policy)), RoleName: aws.String(clusterName)}

		if _, err := clientIam.AttachRolePolicy(context.TODO(), attachRolePolicyInput); err != nil {
			return err
		}
	}

	clientEks := eks.NewFromConfig(cfg)
	// 001. VpcConfigRequest

	listClustersInput := &eks.ListClustersInput{}

	listClusters, err := clientEks.ListClusters(context.TODO(), listClustersInput)
	if err != nil {
		return err
	}

	if containString(listClusters.Clusters, clusterName) == false {
		vpcConfigRequest := &types.VpcConfigRequest{EndpointPrivateAccess: aws.Bool(true), SubnetIds: c.clusterInfo.privateSubnets}

		createClusterInput := &eks.CreateClusterInput{Name: aws.String(clusterName), ResourcesVpcConfig: vpcConfigRequest, RoleArn: aws.String(roleArn)}

		if _, err := clientEks.CreateCluster(context.TODO(), createClusterInput); err != nil {
			return err
		}
	}

	for _idx := 0; _idx < 50; _idx++ {

		// Todo : wait until the cluster become the active
		describeClusterInput := &eks.DescribeClusterInput{Name: aws.String(clusterName)}

		describeCluster, err := clientEks.DescribeCluster(context.TODO(), describeClusterInput)
		if err != nil {
			return err
		}

		if describeCluster.Cluster.Status == "ACTIVE" {
			break
		}
		time.Sleep(1 * time.Minute)
	}

	for _, _cmd := range []string{"which kubectl || curl -L https://storage.googleapis.com/kubernetes-release/release/v1.23.6/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl", "chmod 755 /usr/local/bin/kubectl"} {
		if _, _, err = (*workstation).Execute(ctx, _cmd, true); err != nil {
			return err
		}
	}

	for _, _cmd := range []string{"curl -Lo /usr/local/bin/aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v0.5.9/aws-iam-authenticator_0.5.9_linux_amd64", "chmod 755 /usr/local/bin/aws-iam-authenticator"} {
		if _, _, err = (*workstation).Execute(ctx, _cmd, true); err != nil {
			return err
		}
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf("aws eks update-kubeconfig --name %s", clusterName), false); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, "chmod 600 ~/.kube/config", false); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, "mkdir -p ~/.aws", false); err != nil {
		return err
	}

	_crentials, err := GetAWSCrential()
	if err != nil {
		return err
	}

	err = (*workstation).TransferTemplate(ctx, "templates/config/eks/aws.config.tpl", "~/.aws/config", "0600", map[string]string{"REGION": cfg.Region}, false, 0)
	if err != nil {
		return err
	}

	err = (*workstation).TransferTemplate(ctx, "templates/config/eks/aws.credentials.tpl", "~/.aws/credentials", "0600", *_crentials, false, 0)
	if err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`eksctl utils associate-iam-oidc-provider --cluster %s --approve`, clusterName), false); err != nil {
		return err
	}

	if _, _, err = (*workstation).Execute(ctx, fmt.Sprintf(`eksctl create iamserviceaccount \
   --name ebs-csi-controller-sa \
   --namespace kube-system \
   --cluster %s \
   --attach-policy-arn arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy \
   --approve   --role-only   --role-name AmazonEKS_EBS_CSI_DriverRole_%s`, clusterName, clusterName), false); err != nil {
		return err
	}

	listAddonsInput := &eks.ListAddonsInput{ClusterName: aws.String(clusterName)}
	listAddons, err := clientEks.ListAddons(context.TODO(), listAddonsInput)
	if err != nil {
		return err
	}

	if containString(listAddons.Addons, "aws-ebs-csi-driver") == false {

		createAddonInput := &eks.CreateAddonInput{AddonName: aws.String("aws-ebs-csi-driver"), ClusterName: aws.String(clusterName), ServiceAccountRoleArn: aws.String(fmt.Sprintf("arn:aws:iam::%s:role/AmazonEKS_EBS_CSI_DriverRole_%s", tagAccountID, clusterName))}
		if _, err := clientEks.CreateAddon(context.TODO(), createAddonInput); err != nil {

			return err
		}

	}

	var parallelTasks []Task
	parallelTasks = append(parallelTasks, &DeployEKSNodeGroup{
		pexecutor:         c.pexecutor,
		awsGeneralConfigs: c.awsGeneralConfigs,
		subClusterType:    c.subClusterType,
		clusterInfo:       c.clusterInfo,
		nodeGroupName:     "admin",
	})

	parallelExe := Parallel{ignoreError: false, inner: parallelTasks}
	if err := parallelExe.Execute(ctx); err != nil {
		return err
	}

	controllerExistFlag, err := HelmResourceExist(workstation, "nginx-ingress-controller")
	if controllerExistFlag == false {

		if _, _, err = (*workstation).Execute(ctx, `mkdir -p /opt/helm`, true); err != nil {
			return err
		}

		err = (*workstation).TransferTemplate(ctx, "templates/config/eks/internal.alb.yaml.tpl", "/opt/helm/internal.alb.yaml", "0600", nil, true, 0)
		if err != nil {
			return err
		}

		err = (*workstation).TransferTemplate(ctx, "templates/config/eks/storageClass.yaml.tpl", "/opt/helm/storageClass.yaml", "0600", nil, true, 0)
		if err != nil {
			return err
		}

		_cmds := []string{
			"helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx",
			"helm repo update",
			"helm install nginx-ingress-controller ingress-nginx/ingress-nginx -f /opt/helm/internal.alb.yaml",
			"kubectl create -f /opt/helm/storageClass.yaml",
		}
		for _, _cmd := range _cmds {
			if _, _, err = (*workstation).Execute(ctx, _cmd, false); err != nil {
				return err
			}
		}
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

type DestroyEKS struct {
	pexecutor *ctxt.Executor
	gOpt      operator.Options
}

// Execute implements the Task interface
func (c *DestroyEKS) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	// Remove the openid provider
	clientIam := iam.NewFromConfig(cfg)

	listOpenIDConnectProviders, err := clientIam.ListOpenIDConnectProviders(context.TODO(), &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return err
	}

	for _, openIDConnectProvider := range listOpenIDConnectProviders.OpenIDConnectProviderList {
		listOpenIDConnectProviderTags, err := clientIam.ListOpenIDConnectProviderTags(context.TODO(), &iam.ListOpenIDConnectProviderTagsInput{OpenIDConnectProviderArn: openIDConnectProvider.Arn})
		if err != nil {
			return err
		}

		for _, _tag := range listOpenIDConnectProviderTags.Tags {
			if *_tag.Key == "alpha.eksctl.io/cluster-name" && *_tag.Value == clusterName {
				_, err := clientIam.DeleteOpenIDConnectProvider(context.TODO(), &iam.DeleteOpenIDConnectProviderInput{OpenIDConnectProviderArn: openIDConnectProvider.Arn})
				if err != nil {
					return err
				}
			}
		}
	}

	clientEks := eks.NewFromConfig(cfg)

	listClustersInput := &eks.ListClustersInput{}
	listClusters, err := clientEks.ListClusters(context.TODO(), listClustersInput)
	if err != nil {
		return err
	}

	for _, _cluster := range listClusters.Clusters {
		if _cluster == clusterName {
			workstation, err := GetWSExecutor02(*c.pexecutor, ctx, clusterName, clusterType, c.gOpt.SSHUser, c.gOpt.IdentityFile, true, nil)
			if err != nil {
				return err
			}

			if err = CleanClusterSA(workstation, clusterName); err != nil {
				return err
			}

			// 01. Destroy nginx ingress controller
			controllerExistFlag, err := HelmResourceExist(workstation, "nginx-ingress-controller")
			if controllerExistFlag == true {
				_cmds := []string{
					"helm delete nginx-ingress-controller",
					"kubectl delete -f /opt/helm/storageClass.yaml",
					// fmt.Sprintf("eksctl delete iamserviceaccount --name ebs-csi-controller-sa --namespace kube-system --cluster %s", clusterName),
				}
				for _, _cmd := range _cmds {
					if _, _, err = (*workstation).Execute(ctx, _cmd, false); err != nil {
						return err
					}
				}
			}

			listNodegroupsInput := &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)}
			listNodegroups, err := clientEks.ListNodegroups(context.TODO(), listNodegroupsInput)
			if err != nil {
				return err
			}

			var parallelTasks []Task
			for _, _nodeGroup := range listNodegroups.Nodegroups {
				parallelTasks = append(parallelTasks, &DestroyEKSNodeGroup{
					pexecutor:     c.pexecutor,
					nodeGroupName: _nodeGroup,
				})
			}
			parallelExe := Parallel{ignoreError: false, inner: parallelTasks}
			if err := parallelExe.Execute(ctx); err != nil {
				return err
			}

			deleteClusterInput := &eks.DeleteClusterInput{Name: aws.String(clusterName)}
			_, err = clientEks.DeleteCluster(context.TODO(), deleteClusterInput)
			if err != nil {
				return err
			}

			for _idx := 0; _idx < 100; _idx++ {
				_listClusters, err := clientEks.ListClusters(context.TODO(), listClustersInput)

				if err != nil {
					return err
				}
				if containString(_listClusters.Clusters, clusterName) == false {
					break
				}

				time.Sleep(time.Minute)
			}
		}
	}

	listRoles, err := clientIam.ListRoles(context.TODO(), &iam.ListRolesInput{})
	if err != nil {
		return err
	}

	for _, _role := range listRoles.Roles {
		listRoleTags, err := clientIam.ListRoleTags(context.TODO(), &iam.ListRoleTagsInput{RoleName: _role.RoleName})
		if err != nil {
			return err
		}

		mapTag := make(map[string]string)
		for _, _tag := range listRoleTags.Tags {
			mapTag[*_tag.Key] = *_tag.Value
		}

		var _value string
		var ok bool
		if _value, ok = mapTag["Cluster"]; !ok {
			continue
		}

		if _value != clusterType {
			continue
		}

		if _value, ok = mapTag["Name"]; !ok {
			continue
		}

		if _value != clusterName {
			continue
		}
		if _value, ok = mapTag["Type"]; !ok {
			continue
		}

		if _value != "es" {
			continue
		}

		listAttachedRolePolicies, err := clientIam.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{RoleName: _role.RoleName})
		if err != nil {
			return err
		}

		for _, _rolePolicy := range listAttachedRolePolicies.AttachedPolicies {
			if _, err = clientIam.DetachRolePolicy(context.TODO(), &iam.DetachRolePolicyInput{RoleName: _role.RoleName, PolicyArn: _rolePolicy.PolicyArn}); err != nil {
				return err
			}
		}

		_, err = clientIam.DeleteRole(context.TODO(), &iam.DeleteRoleInput{RoleName: _role.RoleName})
		if err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyEKS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyEKS) String() string {
	return fmt.Sprintf("Echo: Deploying EKS Cluster")
}
