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
	// "strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	// "io/ioutil"
)

type CreateRedshift struct {
	pexecutor   *ctxt.Executor
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateRedshift) Execute(ctx context.Context) error {

	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}
	fmt.Printf("The config is : <%#v> \n\n\n", cfg)
	fmt.Printf("cluster name: <%s>, cluster type: <%s> \n\n\n", clusterName, clusterType)
	fmt.Printf("The context data is : <%#v> \n\n\n", *c.clusterInfo)

	client := redshift.NewFromConfig(cfg)

	describeClusterSubnetGroups, err := client.DescribeClusterSubnetGroups(context.TODO(), &redshift.DescribeClusterSubnetGroupsInput{
		ClusterSubnetGroupName: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterSubnetGroupNotFoundFault" {
				return err
			}
		} else {
			return err
		}
	}

	fmt.Printf("The cluster subnet groups is <%#v> \n\n\n", describeClusterSubnetGroups)

	if describeClusterSubnetGroups == nil {
		createClusterSubnetGroup, err := client.CreateClusterSubnetGroup(context.TODO(), &redshift.CreateClusterSubnetGroupInput{
			ClusterSubnetGroupName: aws.String(clusterName),
			Description:            aws.String(clusterName),
			SubnetIds:              c.clusterInfo.privateSubnets,
			Tags: []types.Tag{
				{
					Key:   aws.String("Cluster"),
					Value: aws.String(clusterType),
				},
				{
					Key:   aws.String("Type"),
					Value: aws.String("redshift"),
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String(clusterName),
				},
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("The error is <%#v> \n\n\n", createClusterSubnetGroup)
	}

	// Cluster Parameter Group
	describeClusterParameterGroups, err := client.DescribeClusterParameterGroups(context.TODO(), &redshift.DescribeClusterParameterGroupsInput{
		ParameterGroupName: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterParameterGroupNotFound" {
				return err
			}
		} else {
			return err
		}
	}

	fmt.Printf("The create cluster parameter group is <%#v> \n\n\n", describeClusterParameterGroups)

	if describeClusterParameterGroups == nil {
		createClusterParameterGroup, err := client.CreateClusterParameterGroup(context.TODO(), &redshift.CreateClusterParameterGroupInput{
			ParameterGroupName:   aws.String(clusterName),
			Description:          aws.String(clusterName),
			ParameterGroupFamily: aws.String("redshift-1.0"),
			Tags: []types.Tag{
				{
					Key:   aws.String("Cluster"),
					Value: aws.String(clusterType),
				},
				{
					Key:   aws.String("Type"),
					Value: aws.String("redshift"),
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String(clusterName),
				},
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("The error is <%#v> \n\n\n", createClusterParameterGroup)
	}

	// Cluster
	describeClusters, err := client.DescribeClusters(context.TODO(), &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterNotFound" {
				return err
			}
		} else {
			return err
		}
	}

	fmt.Printf("The create cluster parameter group is <%#v> \n\n\n", describeClusters)

	if describeClusters == nil {
		createCluster, err := client.CreateCluster(context.TODO(), &redshift.CreateClusterInput{
			ClusterIdentifier:         aws.String(clusterName),
			MasterUserPassword:        aws.String("1234Abcd"),
			MasterUsername:            aws.String("awsadmin"),
			ClusterParameterGroupName: aws.String(clusterName),
			NodeType:                  aws.String("dc2.large"),
			NumberOfNodes:             aws.Int32(1),
			ClusterType:               aws.String("single-node"),
			VpcSecurityGroupIds:       []string{"sg-0f3f21c82a36ea1a2"},
			PubliclyAccessible:        aws.Bool(false),
			ClusterSubnetGroupName:    aws.String(clusterName),
			Tags: []types.Tag{
				{
					Key:   aws.String("Cluster"),
					Value: aws.String(clusterType),
				},
				{
					Key:   aws.String("Type"),
					Value: aws.String("redshift"),
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String(clusterName),
				},
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("The error is <%#v> \n\n\n", createCluster)
	}

	return nil

	// stackInput := &cloudformation.CreateStackInput{
	// 	StackName:    aws.String(clusterName),
	// 	TemplateBody: aws.String(templateBody),
	// 	Parameters:   parameters,
	// 	Tags: []types.Tag{
	// 		{
	// 			Key:   aws.String("Cluster"),
	// 			Value: aws.String(clusterType),
	// 		},
	// 		{
	// 			Key:   aws.String("Type"),
	// 			Value: aws.String("oracle"),
	// 		},
	// 		{
	// 			Key:   aws.String("Name"),
	// 			Value: aws.String(clusterName),
	// 		},
	// 	},
	// }

	// _, err = client.CreateStack(context.TODO(), stackInput)
	// if err != nil {
	// 	fmt.Println("Got an error creating an instance:")
	// 	fmt.Println(err)
	// 	return err
	// }
	// return nil
}

// Rollback implements the Task interface
func (c *CreateRedshift) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRedshift) String() string {
	return fmt.Sprintf("Echo: Create Redshift  ")
}

/******************************************************************************/

type DestroyRedshift struct {
	pexecutor   *ctxt.Executor
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyRedshift) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}

	client := redshift.NewFromConfig(cfg)

	// Cluster
	describeClusters, err := client.DescribeClusters(context.TODO(), &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterNotFound" {
				return err
			}
		} else {
			return err
		}
	}

	fmt.Printf("The create cluster parameter group is <%#v> \n\n\n", describeClusters)

	if describeClusters != nil {
		if _, err := client.DeleteCluster(context.TODO(), &redshift.DeleteClusterInput{
			ClusterIdentifier:        aws.String(clusterName),
			SkipFinalClusterSnapshot: true,
		}); err != nil {
			return err
		}
	}

	describeClusterSubnetGroups, err := client.DescribeClusterSubnetGroups(context.TODO(), &redshift.DescribeClusterSubnetGroupsInput{
		ClusterSubnetGroupName: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterSubnetGroupNotFoundFault" {
				return err
			}
		} else {
			return err
		}
	}

	fmt.Printf("The cluster subnet groups is <%#v> \n\n\n", describeClusterSubnetGroups)

	if describeClusterSubnetGroups != nil {
		if _, err := client.DeleteClusterSubnetGroup(context.TODO(), &redshift.DeleteClusterSubnetGroupInput{
			ClusterSubnetGroupName: aws.String(clusterName),
		}); err != nil {
			return err
		}
	}

	// Cluster Parameter Group
	describeClusterParameterGroups, err := client.DescribeClusterParameterGroups(context.TODO(), &redshift.DescribeClusterParameterGroupsInput{
		ParameterGroupName: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterParameterGroupNotFound" {
				return err
			}
		} else {
			return err
		}
	}

	fmt.Printf("The create cluster parameter group is <%#v> \n\n\n", describeClusterParameterGroups)

	if describeClusterParameterGroups != nil {
		if _, err := client.DeleteClusterParameterGroup(context.TODO(), &redshift.DeleteClusterParameterGroupInput{
			ParameterGroupName: aws.String(clusterName),
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyRedshift) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyRedshift) String() string {
	return fmt.Sprintf("Echo: Destroying CloudFormation")
}

// ----- Oracle
type ListRedshift struct {
	pexecutor     *ctxt.Executor
	tableRedshift *[][]string
}

// Execute implements the Task interface
func (c *ListRedshift) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	// clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}

	client := redshift.NewFromConfig(cfg)

	// Cluster
	describeClusters, err := client.DescribeClusters(context.TODO(), &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterName),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterNotFound" {
				return err
			}
		} else {
			return err
		}
	}

	if describeClusters != nil {
		*(c.tableRedshift) = append(*(c.tableRedshift), []string{
			*describeClusters.Clusters[0].Endpoint.Address,
			fmt.Sprintf("%d", describeClusters.Clusters[0].Endpoint.Port),
			*describeClusters.Clusters[0].DBName,
			*describeClusters.Clusters[0].MasterUsername,
			*describeClusters.Clusters[0].ClusterAvailabilityStatus,
			*describeClusters.Clusters[0].NodeType,
		})
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListRedshift) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListRedshift) String() string {
	return fmt.Sprintf("Echo: List Redshift ")
}
