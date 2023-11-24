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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	// "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
)

type ListAccount struct {
	pexecutor *ctxt.Executor
	account   *string
}

// Execute implements the Task interface
func (c *ListAccount) Execute(ctx context.Context) error {

	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return err
	}

	client := sts.NewFromConfig(cfg)

	getCallerIdentityInput := &sts.GetCallerIdentityInput{}
	getCallerIdentityOutput, err := client.GetCallerIdentity(context.TODO(), getCallerIdentityInput)
	if err != nil {
		return err
	}
	*(c.account) = *(getCallerIdentityOutput.Account)

	return nil
}

// Rollback implements the Task interface
func (c *ListAccount) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ListAccount) String() string {
	return fmt.Sprintf("Echo: List account info ")
}
