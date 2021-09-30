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
    "regexp"

    //"strings"
	"github.com/luyomo/tisample/pkg/candle/ctxt"
	"github.com/luyomo/tisample/pkg/candle/executor"
	"github.com/pingcap/errors"
    "github.com/luyomo/tisample/pkg/logger/log"
)

// Mkdir is used to create directory on the target host
type GcloudCreateInstance struct {
	user string
	host string
}

// Execute implements the Task interface
func (r *GcloudCreateInstance) Execute(ctx context.Context) error {
    fmt.Printf("****** ****** The user is <%s> \n\n\n", r.user)
	local, testErr := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: r.user})
	if testErr != nil {
		return errors.Trace(testErr)
	}
	testctx := ctxt.New(context.Background(), 0)
	testA, testB, testErr := local.Execute(testctx, "/opt/google-cloud-sdk/bin/gcloud compute instances create instance-1 --machine-type=n1-standard-1 --zone=asia-northeast3-b --preemptible --no-restart-on-failure --maintenance-policy=terminate", false)
    fmt.Println(string(testB))
    re := regexp.MustCompile(`\r?\n`)
    testStrA := string(testA)
    testStrA = re.ReplaceAllString(testStrA, " ") 
    log.Infof("This is the test messge to log file")
    fmt.Printf("***** ***** < %s > \n\n\n", testStrA)

	//// gcloud compute instances create instance-1 --machine-type=n1-standard-1 --zone=asia-northeast3-b --preemptible --no-restart-on-failure --maintenance-policy=terminate

	return nil
}

// Rollback implements the Task interface
func (r *GcloudCreateInstance) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (r *GcloudCreateInstance) String() string {
	return fmt.Sprintf("Echo: host=%s ", r.host)
}
