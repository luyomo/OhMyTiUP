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

type BaseRedshiftCluster struct {
	pexecutor *ctxt.Executor

	RedshiftDBInfos        *RedshiftDBInfos
	awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseRedshiftCluster) ClusterExist(redshiftClient *redshift.Client, clusterName string) (bool, error) {
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

func (b *BaseRedshiftCluster) ClusterSubnetGroupNameExist(redshiftClient *redshift.Client, clusterName string) (bool, error) {
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

func (b *BaseRedshiftCluster) ClusterParameterGroupsExist(redshiftClient *redshift.Client, clusterName string) (bool, error) {
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

func (b *BaseRedshiftCluster) ReadRedshiftDBInfo(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)

	// var redshiftDBInfos RedshiftDBInfos

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
			if b.awsRedshiftTopoConfigs == nil {
				b.RedshiftDBInfos.Append(&cluster, "")
			} else {
				b.RedshiftDBInfos.Append(&cluster, b.awsRedshiftTopoConfigs.Password)
			}
		}
	}

	return nil
}

type CreateRedshiftCluster struct {
	BaseRedshiftCluster

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateRedshiftCluster) Execute(ctx context.Context) error {
	clusterName := ctx.Value("clusterName").(string)
	clusterType := ctx.Value("clusterType").(string)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := redshift.NewFromConfig(cfg)

	tags := []types.Tag{
		{Key: aws.String("Cluster"), Value: aws.String(clusterType)},
		{Key: aws.String("Type"), Value: aws.String("redshift")},
		{Key: aws.String("Name"), Value: aws.String(clusterName)},
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
func (c *CreateRedshiftCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateRedshiftCluster) String() string {
	return fmt.Sprintf("Echo: Create Redshift  ")
}

type DestroyRedshiftCluster struct {
	BaseRedshiftCluster
	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *DestroyRedshiftCluster) Execute(ctx context.Context) error {
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

		if err = WaitResourceUntilExpectState(30*time.Second, 5*time.Minute, func() (bool, error) {
			clusterExist, err := c.ClusterExist(client, clusterName)
			return !clusterExist, err
		}); err != nil {
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
func (c *DestroyRedshiftCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyRedshiftCluster) String() string {
	return fmt.Sprintf("Echo: Destroying Redshift")
}

type RedshiftDBInfo struct {
	Host     string `yaml:"host"`
	Port     int32  `yaml:"port"`
	DBName   string `yaml:"db_name"`
	UserName string `yaml:"user_name"`
	Password string `yaml:"password"`
	Status   string
	NodeType string
}

type RedshiftDBInfos struct {
	BaseResourceInfo
}

func (d *RedshiftDBInfos) Append(cluster *types.Cluster, password string) {
	(*d).Data = append((*d).Data, RedshiftDBInfo{
		Host:     *cluster.Endpoint.Address,
		Port:     cluster.Endpoint.Port,
		UserName: *cluster.MasterUsername,
		DBName:   *cluster.DBName,
		Password: password,
		Status:   *cluster.ClusterAvailabilityStatus,
		NodeType: *cluster.NodeType,
	})
}

func (d *RedshiftDBInfos) ToPrintTable() *[][]string {
	tableRedshift := [][]string{{"Endpoint", "Port", "DB Name", "Master User", "State", "Node Type"}}
	for _, _row := range (*d).Data {
		_entry := _row.(RedshiftDBInfo)
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

func (d *RedshiftDBInfos) GetRedshiftDBInfo() (*map[string]string, error) {
	if len((*d).Data) > 1 {
		return nil, errors.New("Multiple redshift db exists")
	}
	if len((*d).Data) == 0 {
		return nil, errors.New("No db exists")
	}

	dbInfo := make(map[string]string)
	for _, _row := range (*d).Data {
		_entry := _row.(RedshiftDBInfo)

		dbInfo["DBHost"] = _entry.Host
		dbInfo["DBPort"] = fmt.Sprintf("%d", _entry.Port)
		dbInfo["DBUser"] = _entry.UserName
		dbInfo["DBPassword"] = _entry.Password
	}
	return &dbInfo, nil
}

type ListRedshiftCluster struct {
	BaseRedshiftCluster
}

// Execute implements the Task interface
func (c *ListRedshiftCluster) Execute(ctx context.Context) error {
	if err := c.ReadRedshiftDBInfo(ctx); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *ListRedshiftCluster) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListRedshiftCluster) String() string {
	return fmt.Sprintf("Echo: List Redshift ")
}

// Deploy Redshift Instance
type DeployRedshiftInstance struct {
	BaseRedshiftCluster

	awsWSConfigs *spec.AwsWSConfigs
	wsExe        *ctxt.Executor
}

// Execute implements the Task interface
func (c *DeployRedshiftInstance) Execute(ctx context.Context) error {
	c.RedshiftDBInfos = &RedshiftDBInfos{}

	if err := c.ReadRedshiftDBInfo(ctx); err != nil {
		return err
	}

	tmpFile := "/tmp/redshift.dbinfo.yaml"
	if err := c.RedshiftDBInfos.WriteIntoConfigFile(tmpFile); err != nil {
		return err
	}

	if err := (*c.wsExe).Transfer(ctx, tmpFile, tmpFile, false, 0); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("sudo mv %s /opt/", tmpFile), true); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, "apt-get install -y postgresql-client-11", true); err != nil {
		return err
	}

	dbInfo, err := c.RedshiftDBInfos.GetRedshiftDBInfo()
	if err != nil {
		return err
	}

	if err := (*c.wsExe).TransferTemplate(ctx, "templates/scripts/run_pg_query.sh.tpl", "/opt/scripts/run_redshift_query", "0755", dbInfo, true, 0); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *DeployRedshiftInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DeployRedshiftInstance) String() string {
	return fmt.Sprintf("Echo: List Redshift ")
}
