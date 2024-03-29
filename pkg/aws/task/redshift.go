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
	smithy "github.com/aws/smithy-go"
	"github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"

	awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
	ws "github.com/luyomo/OhMyTiUP/pkg/workstation"
)

func (b *Builder) CreateRedshiftCluster(pexecutor *ctxt.Executor, subClusterType string, awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs, clusterInfo *ClusterInfo) *Builder {
	clusterInfo.cidr = awsRedshiftTopoConfigs.CIDR

	b.Step(fmt.Sprintf("%s : Creating Basic Resource ... ...", subClusterType),
		NewBuilder().CreateBasicResource(pexecutor, subClusterType, "private", clusterInfo, []int{5439}).Build()).
		Step(fmt.Sprintf("%s : Creating Reshift ... ...", subClusterType), &CreateRedshiftCluster{
			BaseRedshiftCluster: BaseRedshiftCluster{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: "private"}, awsRedshiftTopoConfigs: awsRedshiftTopoConfigs},
			clusterInfo:         clusterInfo,
		})

	return b
}

func (b *Builder) DeployRedshiftInstance(pexecutor *ctxt.Executor, awsWSConfigs *spec.AwsWSConfigs, awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs, wsExe *ctxt.Executor) *Builder {
	b.tasks = append(b.tasks, &DeployRedshiftInstance{
		BaseRedshiftCluster: BaseRedshiftCluster{BaseTask: BaseTask{pexecutor: pexecutor}, awsRedshiftTopoConfigs: awsRedshiftTopoConfigs},
		wsExe:               wsExe,
	})

	return b
}

func (b *Builder) ListRedshiftCluster(pexecutor *ctxt.Executor, redshiftDBInfos *RedshiftDBInfos) *Builder {
	b.tasks = append(b.tasks, &ListRedshiftCluster{
		BaseRedshiftCluster: BaseRedshiftCluster{BaseTask: BaseTask{pexecutor: pexecutor, ResourceData: redshiftDBInfos}},
	})
	return b
}

func (b *Builder) DestroyRedshiftCluster(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyRedshiftCluster{
		BaseRedshiftCluster: BaseRedshiftCluster{BaseTask: BaseTask{pexecutor: pexecutor}},
	})

	b.Step(fmt.Sprintf("%s : Destroying Basic resources ... ...", subClusterType), NewBuilder().DestroyBasicResource(pexecutor, subClusterType).Build())
	return b
}

type BaseRedshiftCluster struct {
	BaseTask
	// pexecutor *ctxt.Executor

	client *redshift.Client // Replace the example to specific service
	// RedshiftDBInfos        *RedshiftDBInfos
	awsRedshiftTopoConfigs *spec.AwsRedshiftTopoConfigs
}

/*
 * Return:
 *   (true, nil): Cluster exist
 *   (false, nil): Cluster does not exist
 *   (false, error): Failed to check
 */
func (b *BaseRedshiftCluster) ClusterExist( /*redshiftClient *redshift.Client, clusterName string*/ ) (bool, error) {
	if _, err := b.client.DescribeClusters(context.TODO(), &redshift.DescribeClustersInput{ClusterIdentifier: aws.String(b.clusterName)}); err != nil {
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

func (b *BaseRedshiftCluster) ClusterSubnetGroupNameExist( /*redshiftClient *redshift.Client, clusterName string*/ ) (bool, error) {
	if _, err := b.client.DescribeClusterSubnetGroups(context.TODO(), &redshift.DescribeClusterSubnetGroupsInput{
		ClusterSubnetGroupName: aws.String(b.clusterName),
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

func (b *BaseRedshiftCluster) ClusterParameterGroupsExist( /*redshiftClient *redshift.Client, clusterName string*/ ) (bool, error) {
	if _, err := b.client.DescribeClusterParameterGroups(context.TODO(), &redshift.DescribeClusterParameterGroupsInput{
		ParameterGroupName: aws.String(b.clusterName),
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

func (b *BaseRedshiftCluster) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = redshift.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &RedshiftDBInfos{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

// func (b *BaseRedshiftCluster) ReadRedshiftDBInfo(ctx context.Context) error {
func (b *BaseRedshiftCluster) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}
	fmt.Printf("The cluster name in the base redshift: %s \n\n\n", b.clusterName)
	describeClusters, err := b.client.DescribeClusters(context.TODO(), &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(b.clusterName),
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
			password := ""
			if b.awsRedshiftTopoConfigs != nil {
				password = b.awsRedshiftTopoConfigs.Password
			}
			_data := ws.RedshiftDBInfo{
				Host:     *cluster.Endpoint.Address,
				Port:     cluster.Endpoint.Port,
				UserName: *cluster.MasterUsername,
				DBName:   *cluster.DBName,
				Password: password,
				Status:   *cluster.ClusterAvailabilityStatus,
				NodeType: *cluster.NodeType,
			}

			b.ResourceData.Append(_data)
		}
	}

	return nil
}

func (d *BaseRedshiftCluster) GetRedshiftDBInfo() (*map[string]string, error) {
	resourceExistFlag, err := d.ResourceData.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExistFlag == false {
		return nil, errors.New("No db exists")
	}

	dbInfo := make(map[string]string)
	for _, _row := range d.ResourceData.GetData() {
		_entry := _row.(ws.RedshiftDBInfo)

		dbInfo["DBHost"] = _entry.Host
		dbInfo["DBPort"] = fmt.Sprintf("%d", _entry.Port)
		dbInfo["DBUser"] = _entry.UserName
		dbInfo["DBPassword"] = _entry.Password
	}
	return &dbInfo, nil
}

type CreateRedshiftCluster struct {
	BaseRedshiftCluster

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateRedshiftCluster) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}

	owner, _, err := GetCallerUser()
	if err != nil {
		return err
	}

	tags := []types.Tag{
		{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)},
		{Key: aws.String("Type"), Value: aws.String(c.subClusterType)},
		{Key: aws.String("Name"), Value: aws.String(c.clusterName)},
		{Key: aws.String("Project"), Value: aws.String(c.clusterName)},
		{Key: aws.String("Owner"), Value: aws.String(owner)},
	}

	clusterSubnetGroupNameExistFlag, err := c.ClusterSubnetGroupNameExist( /*client, clusterName*/ )
	if err != nil {
		return err
	}

	clusterSubnets, err := c.GetSubnetsInfo(0)
	if err != nil {
		return err
	}

	fmt.Printf("The subnets for redshift is <%#v> \n\n\n\n\n\n", clusterSubnets)

	if clusterSubnetGroupNameExistFlag == false {
		if _, err := c.client.CreateClusterSubnetGroup(context.TODO(), &redshift.CreateClusterSubnetGroupInput{
			ClusterSubnetGroupName: aws.String(c.clusterName),
			Description:            aws.String(c.clusterName),
			SubnetIds:              *clusterSubnets,
			Tags:                   tags,
			// SubnetIds:              c.clusterInfo.privateSubnets,
		}); err != nil {
			return err
		}
	}

	clusterParameterGroupsExistFlag, err := c.ClusterParameterGroupsExist( /*client, clusterName*/ )
	if err != nil {
		return err
	}

	if clusterParameterGroupsExistFlag == false {
		if _, err := c.client.CreateClusterParameterGroup(context.TODO(), &redshift.CreateClusterParameterGroupInput{
			ParameterGroupName:   aws.String(c.clusterName),
			Description:          aws.String(c.clusterName),
			ParameterGroupFamily: aws.String("redshift-1.0"),
			Tags:                 tags,
		}); err != nil {
			return err
		}
	}

	// Cluster
	clusterExistFlag, err := c.ClusterExist( /*client, clusterName*/ )
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		securityGroup, err := c.GetSecurityGroup(ThrowErrorIfNotExists)
		if err != nil {
			return err
		}

		if _, err := c.client.CreateCluster(context.TODO(), &redshift.CreateClusterInput{
			ClusterIdentifier:         aws.String(c.clusterName),
			MasterUserPassword:        aws.String(c.awsRedshiftTopoConfigs.Password),
			MasterUsername:            aws.String(c.awsRedshiftTopoConfigs.AdminUser),
			ClusterParameterGroupName: aws.String(c.clusterName),
			NodeType:                  aws.String(c.awsRedshiftTopoConfigs.InstanceType),
			NumberOfNodes:             aws.Int32(1),
			ClusterType:               aws.String(c.awsRedshiftTopoConfigs.ClusterType),
			// VpcSecurityGroupIds:       []string{c.clusterInfo.privateSecurityGroupId},
			VpcSecurityGroupIds:    []string{*securityGroup},
			PubliclyAccessible:     aws.Bool(false),
			ClusterSubnetGroupName: aws.String(c.clusterName),
			Tags:                   tags,
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
	if err := c.init(ctx); err != nil {
		return err
	}

	// clusterName := ctx.Value("clusterName").(string)

	// cfg, err := config.LoadDefaultConfig(context.TODO())
	// if err != nil {
	// 	return err
	// }

	// client := redshift.NewFromConfig(cfg)

	clusterExistFlag, err := c.ClusterExist( /*client, clusterName*/ )
	if err != nil {
		return err
	}

	if clusterExistFlag == true {
		if _, err := c.client.DeleteCluster(context.TODO(), &redshift.DeleteClusterInput{
			ClusterIdentifier:        aws.String(c.clusterName),
			SkipFinalClusterSnapshot: true,
		}); err != nil {
			return err
		}

		if err = awsutils.WaitResourceUntilExpectState(30*time.Second, 5*time.Minute, func() (bool, error) {
			clusterExist, err := c.ClusterExist( /*client, clusterName*/ )
			return !clusterExist, err
		}); err != nil {
			return err
		}

	}

	clusterSubnetGroupNameExistFlag, err := c.ClusterSubnetGroupNameExist( /*client, clusterName*/ )
	if err != nil {
		return err
	}

	if clusterSubnetGroupNameExistFlag == true {

		if _, err := c.client.DeleteClusterSubnetGroup(context.TODO(), &redshift.DeleteClusterSubnetGroupInput{
			ClusterSubnetGroupName: aws.String(c.clusterName),
		}); err != nil {
			return err
		}
	}

	// Cluster Parameter Group
	clusterParameterGroupsExistFlag, err := c.ClusterParameterGroupsExist( /*client, clusterName*/ )
	if err != nil {
		return err
	}

	if clusterParameterGroupsExistFlag == true {
		if _, err := c.client.DeleteClusterParameterGroup(context.TODO(), &redshift.DeleteClusterParameterGroupInput{
			ParameterGroupName: aws.String(c.clusterName),
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

type RedshiftDBInfos struct {
	BaseResourceInfo
}

// func (d *RedshiftDBInfos) Append(cluster *types.Cluster, password string) {
// 	(*d).Data = append((*d).Data, ws.RedshiftDBInfo{
// 		Host:     *cluster.Endpoint.Address,
// 		Port:     cluster.Endpoint.Port,
// 		UserName: *cluster.MasterUsername,
// 		DBName:   *cluster.DBName,
// 		Password: password,
// 		Status:   *cluster.ClusterAvailabilityStatus,
// 		NodeType: *cluster.NodeType,
// 	})
// }

func (d *RedshiftDBInfos) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return nil, nil
	})
}

func (d *RedshiftDBInfos) ToPrintTable() *[][]string {
	tableRedshift := [][]string{{"Endpoint", "Port", "DB Name", "Master User", "State", "Node Type"}}
	for _, _row := range (*d).Data {
		_entry := _row.(ws.RedshiftDBInfo)
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

type ListRedshiftCluster struct {
	BaseRedshiftCluster
}

// Execute implements the Task interface
func (c *ListRedshiftCluster) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}
	// if err := c.ReadRedshiftDBInfo(ctx); err != nil {
	// 	return err
	// }

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

	// awsWSConfigs *spec.AwsWSConfigs
	wsExe *ctxt.Executor
}

// Execute implements the Task interface
func (c *DeployRedshiftInstance) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil {
		return err
	}
	// c.RedshiftDBInfos = &RedshiftDBInfos{}

	// if err := c.ReadRedshiftDBInfo(ctx); err != nil {
	// 	return err
	// }

	tmpFile := "/tmp/redshift.dbinfo.yaml"
	if err := c.ResourceData.WriteIntoConfigFile(tmpFile); err != nil {
		return err
	}
	// if err := c.RedshiftDBInfos.WriteIntoConfigFile(tmpFile); err != nil {
	// 	return err
	// }

	if err := (*c.wsExe).Transfer(ctx, tmpFile, tmpFile, false, 0); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("sudo mv %s /opt/", tmpFile), true); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, "apt-get install -y postgresql-client", true); err != nil {
		return err
	}

	dbInfo, err := c.GetRedshiftDBInfo()
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
