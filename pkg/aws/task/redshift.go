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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type BaseRedshift struct {
	pexecutor *ctxt.Executor
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseRedshift) ClusterExist(redshiftClient *redshift.Client, clusterName string) (bool, error) {
	if _, err := redshiftClient.DescribeClusters(context.TODO(), &redshift.DescribeClustersInput{ClusterIdentifier: aws.String(clusterName)}); err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterNotFound" {
				return false, err
			}
		} else {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (b *BaseRedshift) ClusterSubnetGroupNameExist(redshiftClient *redshift.Client, clusterName string) (bool, error) {
	if _, err := redshiftClient.DescribeClusterSubnetGroups(context.TODO(), &redshift.DescribeClusterSubnetGroupsInput{
		ClusterSubnetGroupName: aws.String(clusterName),
	}); err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterSubnetGroupNotFoundFault" {
				return false, err
			}
		} else {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (b *BaseRedshift) ClusterParameterGroupsExist(redshiftClient *redshift.Client, clusterName string) (bool, error) {
	if _, err := redshiftClient.DescribeClusterParameterGroups(context.TODO(), &redshift.DescribeClusterParameterGroupsInput{
		ParameterGroupName: aws.String(clusterName),
	}); err != nil {

		var ae smithy.APIError
		if errors.As(err, &ae) {
			fmt.Printf("code: %s, message: %s, fault: %s \n\n\n", ae.ErrorCode(), ae.ErrorMessage(), ae.ErrorFault().String())
			if ae.ErrorCode() != "ClusterParameterGroupNotFound" {
				return false, err
			}
		} else {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

type CreateRedshift struct {
	BaseRedshift

	clusterInfo            *ClusterInfo
	awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs
}

// Execute implements the Task interface
func (c *CreateRedshift) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := redshift.NewFromConfig(cfg)

	tags := []types.Tag{
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
	}

	clusterSubnetGroupNameExistFlag, err := c.ClusterSubnetGroupNameExist(client, clusterName)
	if err != nil {
		return err
	}

	if clusterSubnetGroupNameExistFlag == false {
		if _, err := client.CreateClusterSubnetGroup(context.TODO(), &redshift.CreateClusterSubnetGroupInput{
			ClusterSubnetGroupName: aws.String(clusterName),
			Description:            aws.String(clusterName),
			SubnetIds:              c.clusterInfo.privateSubnets,
			Tags:                   tags,
		}); err != nil {
			return err
		}

	}

	clusterParameterGroupsExistFlag, err := c.ClusterParameterGroupsExist(client, clusterName)
	if err != nil {
		return err
	}

	if clusterParameterGroupsExistFlag == false {
		if _, err := client.CreateClusterParameterGroup(context.TODO(), &redshift.CreateClusterParameterGroupInput{
			ParameterGroupName:   aws.String(clusterName),
			Description:          aws.String(clusterName),
			ParameterGroupFamily: aws.String("redshift-1.0"),
			Tags:                 tags,
		}); err != nil {
			return err
		}
	}

	// Cluster
	clusterExistFlag, err := c.ClusterExist(client, clusterName)
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		if _, err := client.CreateCluster(context.TODO(), &redshift.CreateClusterInput{
			ClusterIdentifier:         aws.String(clusterName),
			MasterUserPassword:        aws.String(c.awsRedshiftTopoConfigs.Password),
			MasterUsername:            aws.String(c.awsRedshiftTopoConfigs.AdminUser),
			ClusterParameterGroupName: aws.String(clusterName),
			NodeType:                  aws.String(c.awsRedshiftTopoConfigs.InstanceType),
			NumberOfNodes:             aws.Int32(1),
			ClusterType:               aws.String(c.awsRedshiftTopoConfigs.ClusterType),
			VpcSecurityGroupIds:       []string{c.clusterInfo.privateSecurityGroupId},
			PubliclyAccessible:        aws.Bool(false),
			ClusterSubnetGroupName:    aws.String(clusterName),
			Tags:                      tags,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateRedshift) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRedshift) String() string {
	return fmt.Sprintf("Echo: Create Redshift  ")
}

// func (c *CreateRedshift) Install(ctx context.Context) error {
// 	clusterName := ctx.Value("clusterName").(string)
// 	clusterType := ctx.Value("clusterType").(string)

// 	// 1. Get all the workstation nodes
// 	workstation, err := GetWSExecutor(*c.pexecutor, ctx, clusterName, clusterType, c.awsWSConfigs.UserName, c.awsWSConfigs.KeyFile)
// 	if err != nil {
// 		return err
// 	}

// 	auroraInstanceInfos, err := utils.ExtractInstanceOracleInfo(clusterName, clusterType, "postgres")
// 	if err != nil {
// 		return err
// 	}

// 	var dbInfo DBInfo
// 	dbInfo.DBHost = (*auroraInstanceInfos)[0].EndPointAddress
// 	dbInfo.DBPort = (*auroraInstanceInfos)[0].DBPort
// 	dbInfo.DBUser = (*auroraInstanceInfos)[0].DBUserName
// 	dbInfo.DBPassword = c.awsPostgresConfigs.DBPassword

// 	_, _, err = (*workstation).Execute(ctx, "mkdir -p /opt/scripts", true)
// 	if err != nil {
// 		return err
// 	}

// 	err = (*workstation).TransferTemplate(ctx, "templates/config/db-info.yml.tpl", "/opt/db-info.yml", "0644", dbInfo, true, 0)
// 	if err != nil {
// 		return err
// 	}

// 	err = (*workstation).TransferTemplate(ctx, "templates/scripts/run_pg_query.sh.tpl", "/opt/scripts/run_pg_query", "0755", dbInfo, true, 0)
// 	if err != nil {
// 		return err
// 	}

// 	err = (*workstation).TransferTemplate(ctx, "templates/scripts/run_pg_from_file.sh.tpl", "/opt/scripts/run_pg_from_file", "0755", dbInfo, true, 0)
// 	if err != nil {
// 		return err
// 	}

// 	_, _, err = (*workstation).Execute(ctx, "apt-get update", true)
// 	if err != nil {
// 		return err
// 	}

// 	_, _, err = (*workstation).Execute(ctx, "apt-get install -y postgresql-client-11", true)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

/******************************************************************************/

type DestroyRedshift struct {
	BaseRedshift
	// pexecutor   *ctxt.Executor
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

	clusterExistFlag, err := c.ClusterExist(client, clusterName)
	if err != nil {
		return err
	}

	// Cluster
	clusterExistFlag, err = c.ClusterExist(client, clusterName)
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		if _, err := client.DeleteCluster(context.TODO(), &redshift.DeleteClusterInput{
			ClusterIdentifier:        aws.String(clusterName),
			SkipFinalClusterSnapshot: true,
		}); err != nil {
			return err
		}

		if err = WaitResourceUntilExpectState(30*time.Second, 5*time.Minute, func() (bool, error) { return c.ClusterExist(client, clusterName) }); err != nil {
			return err
		}

	}

	clusterSubnetGroupNameExistFlag, err := c.ClusterSubnetGroupNameExist(client, clusterName)
	if err != nil {
		return err
	}

	if clusterSubnetGroupNameExistFlag == true {

		if _, err := client.DeleteClusterSubnetGroup(context.TODO(), &redshift.DeleteClusterSubnetGroupInput{
			ClusterSubnetGroupName: aws.String(clusterName),
		}); err != nil {
			return err
		}
	}

	// Cluster Parameter Group
	clusterParameterGroupsExistFlag, err := c.ClusterParameterGroupsExist(client, clusterName)
	if err != nil {
		return err
	}

	if clusterParameterGroupsExistFlag == true {
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
	return fmt.Sprintf("Echo: Destroying Redshift")
}

type RedshiftDBInfo struct {
	Host     string
	Port     int32
	DBName   string
	UserName string
	Password string
	Status   string
	NodeType string
}

type RedshiftDBInfos []RedshiftDBInfo

func (d *RedshiftDBInfos) Append(cluster *types.Cluster) {
	*d = append(*d, RedshiftDBInfo{
		Host:     *cluster.Endpoint.Address,
		Port:     cluster.Endpoint.Port,
		UserName: *cluster.MasterUsername,
		DBName:   *cluster.DBName,
		// Password: "1234Abcd",
		Status:   *cluster.ClusterAvailabilityStatus,
		NodeType: *cluster.NodeType,
	})
}

func (d *RedshiftDBInfos) ToPrintTable() *[][]string {
	tableRedshift := [][]string{{"Endpoint", "Port", "DB Name", "Master User", "State", "Node Type"}}
	for _, _entry := range *d {
		tableRedshift = append(tableRedshift, []string{
			_entry.Host,
			fmt.Sprintf("%d", _entry.Port),
			_entry.DBName,
			_entry.UserName,
			_entry.Status,
			_entry.NodeType,
		})
	}
	return &tableRedshift
}

type ListRedshift struct {
	BaseRedshift

	RedshiftDBInfos *RedshiftDBInfos
}

// Execute implements the Task interface
func (c *ListRedshift) Execute(ctx context.Context) error {
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

	if describeClusters != nil {
		for _, cluster := range describeClusters.Clusters {
			c.RedshiftDBInfos.Append(&cluster)
		}
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
