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

package kms

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

func MapTag() *map[string]string {
	return &map[string]string{
		"clusterName":    "Name",
		"clusterType":    "Cluster",
		"subClusterType": "Type",
		"scope":          "Scope",
		"component":      "Component",
	}
}

type KmsAPI struct {
	client *kms.Client

	mapArgs *map[string]string
}

func NewKmsAPI(mapArgs *map[string]string) (*KmsAPI, error) {
	kmsapi := KmsAPI{}

	if mapArgs != nil {
		kmsapi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	kmsapi.client = kms.NewFromConfig(cfg)

	return &kmsapi, nil
}

func (k *KmsAPI) GetKMSKey() (*[]types.KeyListEntry, error) {

	resp, err := k.client.ListKeys(context.TODO(), &kms.ListKeysInput{})
	if err != nil {
		return nil, err
	}

	keyListEntries := []types.KeyListEntry{}

	for _, key := range resp.Keys {

		keyResp, err := k.client.ListResourceTags(context.TODO(), &kms.ListResourceTagsInput{KeyId: key.KeyId})
		if err != nil {
			return nil, err
		}

		matchedTag := 3
		for _, tag := range keyResp.Tags {
			switch {
			case *tag.TagKey == "Name" && *tag.TagValue == (*k.mapArgs)["clusterName"]:
				matchedTag = matchedTag - 1
			case *tag.TagKey == "Cluster" && *tag.TagValue == (*k.mapArgs)["clusterType"]:
				matchedTag = matchedTag - 1
			case *tag.TagKey == "Type" && *tag.TagValue == (*k.mapArgs)["subClusterType"]:
				matchedTag = matchedTag - 1
			}
		}

		if matchedTag == 0 {
			keyListEntries = append(keyListEntries, key)
		}
	}

	if len(keyListEntries) > 1 {
		return nil, errors.New("Multiple kms keys found.")
	}

	if len(keyListEntries) == 0 {
		return nil, nil
	} else {
		return &keyListEntries, nil
	}
}

func (c *KmsAPI) makeTags() *[]types.Tag {
	var tags []types.Tag
	if c.mapArgs == nil {
		return &tags
	}

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			tags = append(tags, types.Tag{TagKey: aws.String(tagName), TagValue: aws.String(tagValue)})
		}
	}

	return &tags
}
