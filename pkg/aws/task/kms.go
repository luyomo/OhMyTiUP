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
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	kmsapi "github.com/luyomo/OhMyTiUP/pkg/aws/utils/kms"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
	// awsutils "github.com/luyomo/OhMyTiUP/pkg/aws/utils"
)

func (b *Builder) CreateKMS(subClusterType string) *Builder {
	b.tasks = append(b.tasks, &CreateKMS{
		BaseKMS: BaseKMS{BaseTask: BaseTask{subClusterType: subClusterType}},
	})
	return b
}

func (b *Builder) DestroyKMS(subClusterType string) *Builder {
	b.tasks = append(b.tasks, &DestroyKMS{
		BaseKMS: BaseKMS{BaseTask: BaseTask{subClusterType: subClusterType}},
	})
	return b
}

/******************************************************************************/

type KMSInfo struct {
	BaseResourceInfo
}

func (d *KMSInfo) ToPrintTable() *[][]string {
	tableVPC := [][]string{{"Cluster Name"}}
	for _, _row := range d.Data {
		// _entry := _row.(VPC)
		// tableVPC = append(tableVPC, []string{
		// 	// *_entry.PolicyName,
		// })

		log.Infof("%#v", _row)
	}
	return &tableVPC
}

func (d *KMSInfo) GetResourceArn(throwErr ThrowErrorFlag) (*string, error) {
	return d.BaseResourceInfo.GetResourceArn(throwErr, func(_data interface{}) (*string, error) {
		return _data.(types.KeyListEntry).KeyArn, nil
	})
}

/******************************************************************************/
type BaseKMS struct {
	BaseTask

	// The below variables are initialized in the init() function
	client *kms.Client // Replace the example to specific service
}

func (b *BaseKMS) init(ctx context.Context) error {
	if ctx != nil {
		b.clusterName = ctx.Value("clusterName").(string)
		b.clusterType = ctx.Value("clusterType").(string)
	}

	// Client initialization
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	b.client = kms.NewFromConfig(cfg) // Replace the example to specific service

	// Resource data initialization
	if b.ResourceData == nil {
		b.ResourceData = &KMSInfo{}
	}

	if err := b.readResources(); err != nil {
		return err
	}

	return nil
}

func (b *BaseKMS) readResources() error {
	if err := b.ResourceData.Reset(); err != nil {
		return err
	}

	mapArgs := make(map[string]string)
	mapArgs["clusterName"] = b.clusterName
	mapArgs["clusterType"] = b.clusterType
	mapArgs["subClusterType"] = b.subClusterType

	kmsapi, err := kmsapi.NewKmsAPI(&mapArgs)
	if err != nil {
		return err
	}

	kmsKeys, err := kmsapi.GetKMSKey()
	if err != nil {
		return err
	}
	if kmsKeys == nil {
		return nil
	}

	for _, kmsKey := range *kmsKeys {
		b.ResourceData.Append(kmsKey)
	}

	return nil
}

func (b *BaseKMS) GetKeyId() (*string, error) {
	_data := b.ResourceData.GetData()
	if len(_data) == 0 {
		return nil, errors.New("No Key ID found")
	}
	return _data[0].(types.KeyListEntry).KeyId, nil

}

/******************************************************************************/
type CreateKMS struct {
	BaseKMS
}

// Execute implements the Task interface
func (c *CreateKMS) Execute(ctx context.Context) error {
	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	clusterExistFlag, err := c.ResourceData.ResourceExist()
	if err != nil {
		return err
	}

	if clusterExistFlag == false {
		var tags []types.Tag
		tags = append(tags, types.Tag{TagKey: aws.String("Name"), TagValue: aws.String(c.clusterName)})
		tags = append(tags, types.Tag{TagKey: aws.String("Cluster"), TagValue: aws.String(c.clusterType)})
		tags = append(tags, types.Tag{TagKey: aws.String("Type"), TagValue: aws.String(c.subClusterType)})

		if _, err = c.client.CreateKey(context.TODO(), &kms.CreateKeyInput{
			KeySpec:  types.KeySpecSymmetricDefault,
			KeyUsage: types.KeyUsageTypeEncryptDecrypt,
			Tags:     tags,
		}); err != nil {
			return err
		}
	}

	if err := c.createAliasName(ctx); err != nil {
		return err
	}

	return nil
}

func (c *CreateKMS) createAliasName(ctx context.Context) error {

	if err := c.init(ctx); err != nil { // ClusterName/ClusterType and client initialization
		return err
	}

	_data := c.ResourceData.GetData()
	keyId := _data[0].(types.KeyListEntry).KeyId

	resp, err := c.client.ListAliases(context.TODO(), &kms.ListAliasesInput{KeyId: keyId})
	if err != nil {
		return err
	}

	if len(resp.Aliases) == 0 {
		if _, err := c.client.CreateAlias(context.TODO(), &kms.CreateAliasInput{
			AliasName:   aws.String(fmt.Sprintf("alias/%s-%s", c.clusterName, c.clusterType)),
			TargetKeyId: keyId,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Rollback implements the Task interface
func (c *CreateKMS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CreateKMS) String() string {
	return fmt.Sprintf("Echo: Create VPC ... ...  ")
}

type DestroyKMS struct {
	BaseKMS
}

// Execute implements the Task interface
func (c *DestroyKMS) Execute(ctx context.Context) error {
	c.init(ctx) // ClusterName/ClusterType and client initialization

	for _, rdsExportS3 := range c.ResourceData.GetData() {
		fmt.Printf("The task : <%#v> \n\n\n", rdsExportS3)

		// if _, err := c.client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
		// 	VpcId: vpc.(types.Vpc).VpcId,
		// }); err != nil {
		// 	return err
		// }
	}

	return nil
}

// Rollback implements the Task interface
func (c *DestroyKMS) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *DestroyKMS) String() string {
	return fmt.Sprintf("Echo: Destroying VPC")
}
