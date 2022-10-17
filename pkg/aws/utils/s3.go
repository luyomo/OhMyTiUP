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

package utils

import (
	"context"
	// "time"
	// "fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	// "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func GetS3BucketLocation(bucketName string) (string, error) {
	_ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(_ctx)
	if err != nil {
		return "", err
	}

	client := s3.NewFromConfig(cfg)

	getBucketLocationInput := &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	}

	getBucketLocationOutput, err := client.GetBucketLocation(context.TODO(), getBucketLocationInput)
	if err != nil {
		return "", err
	}
	// fmt.Printf("The bucket information is <%#v> \n\n\n\n", getBucketLocationOutput)

	// fmt.Printf("The localtion is <%s> \n\n\n", getBucketLocationOutput.LocationConstraint)

	return string(getBucketLocationOutput.LocationConstraint), nil
}
