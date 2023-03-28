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
	"runtime/debug"

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
func (b *Builder) CreateSubnets(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool, clusterInfo *ClusterInfo) *Builder {
	var scope string
	if isPrivate == true {
		scope = "private"
	} else {
		scope = "public"
	}
	b.tasks = append(b.tasks, &CreateSubnets{
		BaseSubnets: BaseSubnets{
			BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: scope},
		},
		clusterInfo: clusterInfo,
	})
	return b
}

func (b *Builder) DestroySubnets(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool) *Builder {
	var scope string
	if isPrivate == true {
		scope = "private"
	} else {
		scope = "public"
	}

	b.tasks = append(b.tasks, &DestroySubnets{
		BaseSubnets: BaseSubnets{
			BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: scope},
		},
	})
	return b
}

func (b *Builder) ListSubnets(pexecutor *ctxt.Executor, subClusterType string, isPrivate bool) *Builder {
	var scope string
	if isPrivate == true {
		scope = "private"
	} else {
		scope = "public"
	}

	b.tasks = append(b.tasks, &ListSubnets{
		BaseSubnets: BaseSubnets{
			BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: scope},
		},
	})
	return b
}

// func (b *Builder) CreateSubnet() *Builder {
// 	b.tasks = append(b.tasks, &CreateSubnets{})
// 	return b
// }

// func (b *Builder) ListSubnet() *Builder {
// 	b.tasks = append(b.tasks, &ListSubnets{})
// 	return b
// }

// func (b *Builder) DestroySubnet() *Builder {
// 	b.tasks = append(b.tasks, &DestroySubnets{})
// 	return b
// }

/******************************************************************************/

type SubnetsInfo struct {
	BaseResourceInfo
}

func (d *SubnetsInfo) ToPrintTable() *[][]string {
	tableSubnet := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(Subnet)
		// tableSubnet = append(tableSubnet, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableSubnet
}

func (d *SubnetsInfo) GetResourceArn() (*string, error) {
	// TODO: Implement
	_, err := d.ResourceExist()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

/******************************************************************************/
type BaseSubnets struct {
	BaseTask

	// ResourceData ResourceData
	// subClusterType string

	// scope string
	// isPrivate bool `default:false`
	/* awsExampleTopoConfigs *spec.AwsExampleTopoConfigs */ // Replace the config here

	// The below variables are initialized in the init() function
	client *ec2.Client // Replace the example to specific service
}

func (b *BaseSubnets) init(ctx context.Context) error {
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
		b.ResourceData = &SubnetsInfo{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseSubnets) GetSubnets(numSubnets int) (*[]string, error) {
	_data := b.ResourceData.GetData()
	if _data == nil {
		// If no actual number subnets are specified, it's okay to return empty list
		if numSubnets == 0 {
			return &[]string{}, nil
		} else {
			debug.PrintStack()
			return nil, errors.New("No valid subnets found")
		}
	}

	var subnets []string
	for idx, _entry := range _data {
		_subnet := _entry.(types.Subnet)
		if numSubnets == 0 || idx < numSubnets {
			subnets = append(subnets, *_subnet.SubnetId)
		}
	}

	return &subnets, nil
}

func (b *BaseSubnets) getUsedZones() (*[]string, error) {
	_data := b.ResourceData.GetData()
	if _data == nil {
		// If no actual number subnets are specified, it's okay to return empty list
		return &[]string{}, nil
	}

	var zones []string
	for _, _entry := range _data {
		_subnet := _entry.(types.Subnet)
		zones = append(zones, *_subnet.AvailabilityZoneId)
	}

	return &zones, nil
}

func (b *BaseSubnets) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	var filters []types.Filter
	fmt.Printf("Name: %s, Cluster: %s, Type: %s, Scope: %s \n\n\n\n\n", b.clusterName, b.clusterType, b.subClusterType, b.scope)
	filters = append(filters, types.Filter{Name: aws.String("tag:Name"), Values: []string{b.clusterName}})
	filters = append(filters, types.Filter{Name: aws.String("tag:Cluster"), Values: []string{b.clusterType}})

	// If the subClusterType is not specified, it is called from destroy to remove all the security group
	if b.subClusterType != "" {
		filters = append(filters, types.Filter{Name: aws.String("tag:Type"), Values: []string{b.subClusterType}})
	}

	if b.scope != "" {
		filters = append(filters, types.Filter{Name: aws.String("tag:Scope"), Values: []string{b.scope}})
	}

	resp, err := b.client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{Filters: filters})
	if err != nil {
		return err
	}

	for _, subnet := range resp.Subnets {
		b.ResourceData.Append(subnet)
	}
	return nil
}

/******************************************************************************/
type CreateSubnets struct {
	BaseSubnets

	clusterInfo *ClusterInfo
}

// Execute implements the Task interface
func (c *CreateSubnets) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	vpcId, err := c.GetVpcItem("VpcId")
	if err != nil {
		return err
	}

	cidrBlock, err := c.GetVpcItem("CidrBlock")
	if err != nil {
		return err
	}

	usedZones, err := c.getUsedZones()
	if err != nil {
		return err
	}

	zones4subnet, err := c.getAvailableZones(usedZones)
	if err != nil {
		return err
	}

	for idx, zone := range *zones4subnet {
		tags := c.MakeEC2Tags()

		if _, err = c.client.CreateSubnet(context.TODO(), &ec2.CreateSubnetInput{
			VpcId:            vpcId,
			AvailabilityZone: zone.ZoneName,
			CidrBlock:        aws.String(getNextCidr(*cidrBlock, len(*usedZones)+idx+1)),
			TagSpecifications: []types.TagSpecification{
				types.TagSpecification{
					ResourceType: types.ResourceTypeSubnet,
					Tags:         *tags,
				},
			},
		}); err != nil {
			return err
		}
	}

	if err := c.associateSubnetToRouteTable(); err != nil {
		return err
	}
	return nil
}

func (c *CreateSubnets) associateSubnetToRouteTable() error {
	if err := c.readResources(); err != nil {
		return err
	}

	routeTable, err := c.GetRouteTable()
	if err != nil {
		return err
	}
	fmt.Printf("The return value is <%#v> \n\n\n", routeTable)
	fmt.Printf("The associate is <%#v> \n\n\n", routeTable.Associations)

	for _, _entry := range c.ResourceData.GetData() {
		_subnet := _entry.(types.Subnet)
		fmt.Printf("The subnet is <%s>  and <%s> \n\n\n", *_subnet.SubnetId)

		if _, err = c.client.AssociateRouteTable(context.TODO(), &ec2.AssociateRouteTableInput{
			RouteTableId: routeTable.RouteTableId,
			SubnetId:     _subnet.SubnetId,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (c *CreateSubnets) getAvailableZones(usedSubnetList *[]string) (*[]types.AvailabilityZone, error) {
	availableZones, err := c.client.DescribeAvailabilityZones(context.TODO(), &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return nil, err
	}

	var retZones []types.AvailabilityZone

	fmt.Printf("The subnet num is <%d> \n\n\n", c.clusterInfo.subnetsNum)
	fmt.Printf("The used subnets are <%#v> \n\n\n\n\n", *usedSubnetList)

	for _, zone := range availableZones.AvailabilityZones {

		// fmt.Printf("AZ: <%#v> vs <%s> \n\n\n\n\n", c.clusterInfo.excludedAZ, *zone.ZoneName)
		if ListContainElement(c.clusterInfo.excludedAZ, *zone.ZoneName) == true {
			continue
		}
		// fmt.Printf("not matched: <%#v> vs <%s> \n\n\n\n\n", c.clusterInfo.excludedAZ, *zone.ZoneName)

		if len(c.clusterInfo.includedAZ) > 0 && ListContainElement(c.clusterInfo.includedAZ, *zone.ZoneName) == false {
			continue
		}

		if ListContainElement(*usedSubnetList, *zone.ZoneId) == true {
			continue
		}

		if c.clusterInfo.subnetsNum > 0 && len(retZones) >= c.clusterInfo.subnetsNum-len(*usedSubnetList) {
			break
		}

		retZones = append(retZones, zone)

	}

	return &retZones, nil

}

// Rollback implements the Task interface
func (c *CreateSubnets) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateSubnets) String() string {
	return fmt.Sprintf("Echo: Create Subnet ... ...  ")
}

type DestroySubnets struct {
	BaseSubnets
}

// Execute implements the Task interface
func (c *DestroySubnets) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** DestroySubnet ****** \n\n\n")

	c.init(ctx) // ClusterName/ClusterType and client initialization

	for _, subnet := range c.ResourceData.GetData() {
		fmt.Printf("The subnets is <%#v> \n\n\n", subnet)

		if _, err := c.client.DeleteSubnet(context.Background(), &ec2.DeleteSubnetInput{
			SubnetId: subnet.(types.Subnet).SubnetId,
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

	// return nil
}

// Rollback implements the Task interface
func (c *DestroySubnets) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroySubnets) String() string {
	return fmt.Sprintf("Echo: Destroying Subnet")
}

type ListSubnets struct {
	BaseSubnets
}

// Execute implements the Task interface
func (c *ListSubnets) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	fmt.Printf("***** ListSubnets ****** \n\n\n")

	return nil
}

// Rollback implements the Task interface
func (c *ListSubnets) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListSubnets) String() string {
	return fmt.Sprintf("Echo: List  ")
}
