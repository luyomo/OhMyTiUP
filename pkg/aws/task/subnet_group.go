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
func (b *Builder) CreateSubnets(pexecutor *ctxt.Executor, subClusterType string, network NetworkType, clusterInfo *ClusterInfo) *Builder {
	b.tasks = append(b.tasks, &CreateSubnets{
		BaseSubnets: BaseSubnets{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: network}},
		clusterInfo: clusterInfo,
	})

	// // If it's the nat network, generate the private network as well.
	// if network == NetworkTypeNAT {
	// 	b.tasks = append(b.tasks, &CreateSubnets{
	// 		BaseSubnets: BaseSubnets{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: NetworkTypePrivate}},
	// 		clusterInfo: clusterInfo,
	// 	})
	// }

	return b
}

func (b *Builder) DestroySubnets(pexecutor *ctxt.Executor, subClusterType string, network NetworkType) *Builder {
	b.tasks = append(b.tasks, &DestroySubnets{
		BaseSubnets: BaseSubnets{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: network}},
	})
	return b
}

func (b *Builder) ListSubnets(pexecutor *ctxt.Executor, subClusterType string, network NetworkType) *Builder {

	b.tasks = append(b.tasks, &ListSubnets{
		BaseSubnets: BaseSubnets{BaseTask: BaseTask{pexecutor: pexecutor, subClusterType: subClusterType, scope: network}},
	})
	return b
}

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

func (d *SubnetsInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	// TODO: Implement
	// _, err := d.ResourceExist()
	// if err != nil {
	// 	return nil, err
	// }

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

	filters := b.MakeEC2Filters()

	resp, err := b.client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{Filters: *filters})
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

		// If the NAT network is specified, specify the cidr from 20
		// while 01 is started from general case.
		var _cidrBlock string
		if c.scope == NetworkTypeNAT {
			_cidrBlock = getNextCidr(*cidrBlock, 20+len(*usedZones)+idx+1)
		} else if c.scope == NetworkTypePublic {
			_cidrBlock = getNextCidr(*cidrBlock, 30+len(*usedZones)+idx+1)
		} else {
			_cidrBlock = getNextCidr(*cidrBlock, len(*usedZones)+idx+1)
		}

		if _, err = c.client.CreateSubnet(context.TODO(), &ec2.CreateSubnetInput{
			VpcId:            vpcId,
			AvailabilityZone: zone.ZoneName,
			CidrBlock:        aws.String(_cidrBlock),
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

	for _, _entry := range c.ResourceData.GetData() {
		_subnet := _entry.(types.Subnet)

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

	var subnetsNum int
	if c.scope == "nat" {
		subnetsNum = 1
	} else {
		subnetsNum = c.clusterInfo.subnetsNum
	}

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

		if subnetsNum > 0 && len(retZones) >= subnetsNum-len(*usedSubnetList) {
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

	for _, subnet := range c.ResourceData.GetData() {
		if _, err := c.client.DeleteSubnet(context.Background(), &ec2.DeleteSubnetInput{
			SubnetId: subnet.(types.Subnet).SubnetId,
		}); err != nil {
			return err
		}
	}

	return nil
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
    if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
        return err
    }

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
