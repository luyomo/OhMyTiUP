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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func GetAWSCrential() (*aws.Credentials, error) {
	_ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(_ctx)
	if err != nil {
		return nil, err
	}

	_crentials, err := cfg.Credentials.Retrieve(_ctx)
	return &_crentials, nil
}

func GetDefaultRegion() (*string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	return &cfg.Region, nil
}
