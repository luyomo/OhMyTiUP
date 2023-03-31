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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	// "github.com/aws/smithy-go"
	// "github.com/luyomo/OhMyTiUP/pkg/aws/spec"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// "go.uber.org/zap"
)

/******************************************************************************/
func (b *Builder) CreateSecurityGroup(pexecutor *ctxt.Executor, subClusterType string, network NetworkType, openPorts []int) *Builder {

	// if network == NetworkTypeNAT {
	// 	b.tasks = append(b.tasks, &CreateSecurityGroup{
	// 		BaseSecurityGroup: BaseSecurityGroup{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypePrivate}},
	// 		openPorts:         openPorts,
	// 	})
	// }
	b.tasks = append(b.tasks, &CreateSecurityGroup{
		BaseSecurityGroup: BaseSecurityGroup{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: network}},
		openPorts:         openPorts,
	})
	return b
}

func (b *Builder) ListSecurityGroup(pexecutor *ctxt.Executor, tableSecurityGroups *[][]string) *Builder {
	b.tasks = append(b.tasks, &ListSecurityGroup{
		BaseSecurityGroup:   BaseSecurityGroup{BaseTask: BaseTask{pexecutor: pexecutor}},
		tableSecurityGroups: tableSecurityGroups,
	})
	return b
}

func (b *Builder) DestroySecurityGroup(pexecutor *ctxt.Executor, subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroySecurityGroup{
		BaseSecurityGroup: BaseSecurityGroup{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType}},
	})
	return b
}

/******************************************************************************/

type SecurityGroupsInfo struct {
	BaseResourceInfo
}

func (d *SecurityGroupsInfo) ToPrintTable() *[][]string {
	tableSecurityGroup := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(SecurityGroup)
		// tableSecurityGroup = append(tableSecurityGroup, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableSecurityGroup
}

func (d *SecurityGroupsInfo) GetResourceArn() (*string, error) {
	// TODO: Implement
	resourceExists, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}
	if resourceExists == false {
		return nil, errors.New("No resource(security group) found")
	}

	return (d.Data[0]).(types.SecurityGroup).GroupId, nil
}

/******************************************************************************/
type BaseSecurityGroup struct {
	BaseTask

	ResourceData ResourceData
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
	// subClusterType string
	// scope          string
	// isPrivate      bool `default:false`

}

func (b *BaseSecurityGroup) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = ec2.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &SecurityGroupsInfo{}
	}
	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseSecurityGroup) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}
	filters := b.MakeEC2Filters()
	// var filters []types.Filter
	// filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{b.clusterName}})
	// filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{b.clusterType}})

	// // If the subClusterType is not specified, it is called from destroy to remove all the security group
	// if b.subClusterType != "" {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{b.subClusterType}})
	// }

	// if b.scope != "" {
	// 	filters = append(filters, types.Filter{Name: aws.String("tag:Scope"), Values: []string{string(b.scope)}})
	// }
	fmt.Printf("------ security group: <%#v> \n\n\n\n", filters)

	resp, err := b.client.DescribeSecurityGroups(context.TODO(), &ec2.DescribeSecurityGroupsInput{Filters: *filters})
	if err != nil {
		return err
	}

	for _, securityGroup := range resp.SecurityGroups {
		b.ResourceData.Append(securityGroup)
	}

	return nil
}

/******************************************************************************/
type CreateSecurityGroup struct {
	BaseSecurityGroup

	openPorts []int
	// openPortsPublic  []int
	// openPortsPrivate []int
}

// Execute implements the Task interface
func (c *CreateSecurityGroup) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	// if err := c.readResources(); err != nil {
	// 	return err
	// }

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		tags := c.MakeEC2Tags()

		// var tags []types.Tag
		// tags = append(tags, types.Tag{Key: aws.String("Name"), Value: aws.String(c.clusterName)})
		// tags = append(tags, types.Tag{Key: aws.String("Cluster"), Value: aws.String(c.clusterType)})

		// // If the subClusterType is not specified, it is called from destroy to remove all the security group
		// if c.subClusterType != "" {
		// 	tags = append(tags, types.Tag{Key: aws.String("Type"), Value: aws.String(c.subClusterType)})
		// }

		// if c.scope != "" {
		// 	tags = append(tags, types.Tag{Key: aws.String("Scope"), Value: aws.String(string(c.scope))})
		// }

		vpcId, err := c.GetVpcItem("VpcId")
		if err != nil {
			return err
		}

		if _, err = c.client.CreateSecurityGroup(context.TODO(), &ec2.CreateSecurityGroupInput{
			GroupName: aws.String(fmt.Sprintf("%s-%s", c.clusterName, c.scope)),
			VpcId:     vpcId,
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeSecurityGroup,
					Tags:         *tags,
				},
			},
			Description: aws.String(c.clusterName),
		}); err != nil {
			return err
		}
	}

	if err := c.readResources(); err != nil {
		return err
	}

	if err := c.addOpenPorts(); err != nil {
		return err
	}

	return nil
}

func (c *CreateSecurityGroup) addOpenPorts() error {
	securityGroupID, err := c.ResourceData.GetResourceArn()
	if err != nil {
		return err
	}

	for _, port := range c.openPorts {
		if _, err = c.client.AuthorizeSecurityGroupIngress(context.TODO(), &ec2.AuthorizeSecurityGroupIngressInput{
			CidrIp:     aws.String("0.0.0.0/0"),
			GroupId:    securityGroupID,
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(int32(port)),
			ToPort:     aws.Int32(int32(port)),
		}); err != nil {
			// TODO: Added check before
			// return err
		}
	}

	cidr, err := c.GetVpcItem("CidrBlock")
	if err != nil {
		return err
	}

	if _, err = c.client.AuthorizeSecurityGroupIngress(context.TODO(), &ec2.AuthorizeSecurityGroupIngressInput{
		CidrIp:     cidr,
		GroupId:    securityGroupID,
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(0),
		ToPort:     aws.Int32(65535),
	}); err != nil {
		// TODO: Added check before
		// return err
	}

	if _, err = c.client.AuthorizeSecurityGroupIngress(context.TODO(), &ec2.AuthorizeSecurityGroupIngressInput{
		CidrIp:     cidr,
		GroupId:    securityGroupID,
		IpProtocol: aws.String("icmp"),
		FromPort:   aws.Int32(-1),
		ToPort:     aws.Int32(-1),
	}); err != nil {
		// TODO: Add check before
		// return err
	}

	// if _, err = c.client.AuthorizeSecurityGroupIngress(context.TODO(), &ec2.AuthorizeSecurityGroupIngressInput{
	// 	CidrIp:     aws.String("0.0.0.0/0"),
	// 	GroupId:    securityGroupID,
	// 	IpProtocol: aws.String("tcp"),
	// }); err != nil {
	// 	return err
	// }

	return nil
}

// Rollback implements the Task interface
func (c *CreateSecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateSecurityGroup) String() string {
	return fmt.Sprintf("Echo: Create SecurityGroup ... ...  ")
}

type DestroySecurityGroup struct {
	BaseSecurityGroup

	subClusterType string
}

// Execute implements the Task interface
func (c *DestroySecurityGroup) Execute(ctx context.Context) error {
	fmt.Printf("***** DestroySecurityGroup ****** \n\n\n")

	c.init(ctx) // ClusterName/ClusterType and client initialization

	for _, securityGroup := range c.ResourceData.GetData() {
		fmt.Printf("The security group is <%#v> \n\n\n", securityGroup)

		if _, err := c.client.DeleteSecurityGroup(context.Background(), &ec2.DeleteSecurityGroupInput{
			GroupId: securityGroup.(types.SecurityGroup).GroupId,
		}); err != nil {
			return err
		}
	}
	return nil
	// clusterExistFlag, err := c.ResourceData.ResourceExist()
	// if err != nil {
	// 	return err
	// }

	// if clusterExistFlag == true {
	// 	// TODO: Destroy the cluster
	// }

	return nil
}

// Rollback implements the Task interface
func (c *DestroySecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroySecurityGroup) String() string {
	return fmt.Sprintf("Echo: Destroying SecurityGroup")
}

type ListSecurityGroup struct {
	BaseSecurityGroup

	tableSecurityGroups *[][]string
}

// Execute implements the Task interface
func (c *ListSecurityGroup) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListSecurityGroup ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListSecurityGroup) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListSecurityGroup) String() string {
	return fmt.Sprintf("Echo: List  ")
}
