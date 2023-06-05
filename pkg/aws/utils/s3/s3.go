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

package s3

import (
	"context"
	// "fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	// "github.com/luyomo/OhMyTiUP/pkg/utils"
	// "go.uber.org/zap"
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

type S3API struct {
	client *s3.Client

	mapArgs *map[string]string
}

func NewS3API(mapArgs *map[string]string) (*S3API, error) {
	s3api := S3API{}

	if mapArgs != nil {
		s3api.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	s3api.client = s3.NewFromConfig(cfg)

	return &s3api, nil
}

func (c *S3API) GetObject(bucket, key string) error {
	_, err := c.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *S3API) DeleteObject(bucket, prefix string) error {

	objects, err := c.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return err
	}

	var objectIds []types.ObjectIdentifier
	for _, file := range objects.Contents {
		objectIds = append(objectIds, types.ObjectIdentifier{Key: file.Key})
	}

	if _, err := c.client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: objectIds},
	}); err != nil {
		return err
	}

	return nil
}

func (c *S3API) makeTags() *[]types.Tag {
	var tags []types.Tag
	if c.mapArgs == nil {
		return &tags
	}

	for key, tagName := range *(MapTag()) {
		if tagValue, ok := (*c.mapArgs)[key]; ok {
			tags = append(tags, types.Tag{Key: aws.String(tagName), Value: aws.String(tagValue)})
		}
	}

	return &tags
}

// func (c *S3API) makeFilters() *[]types.Filter {
// 	var filters []types.Filter
// 	if c.mapArgs == nil {
// 		return &filters
// 	}

// 	for key, tagName := range *(MapTag()) {
// 		if tagValue, ok := (*c.mapArgs)[key]; ok {
// 			filters = append(filters, types.Filter{Name: aws.String("tag:" + tagName), Values: []string{tagValue}})
// 		}
// 	}

// 	return &filters
// }
