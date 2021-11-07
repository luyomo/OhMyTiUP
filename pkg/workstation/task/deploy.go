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
    //"regexp"

    //"strings"
	//"github.com/luyomo/tisample/pkg/workstation/ctxt"
	//"github.com/luyomo/tisample/pkg/workstation/executor"
	//"github.com/pingcap/errors"
    //"github.com/luyomo/tisample/pkg/logger/log"

     //"golang.org/x/oauth2/google"
     //"google.golang.org/grpc/metadata"
     "google.golang.org/api/iterator"
     "google.golang.org/api/option"
     compute "cloud.google.com/go/compute/apiv1"
     computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
)

// Mkdir is used to create directory on the target host
type Deploy struct {
	user string
	host string
}

// Execute implements the Task interface
func (r *Deploy) Execute(ctx context.Context) error {
    //fmt.Printf("****** ****** The user is <%s> \n\n\n", r.user)
	//local, testErr := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: r.user})
	//if testErr != nil {
	//	return errors.Trace(testErr)
	//}
	//testctx := ctxt.New(context.Background(), 0)
	////testA, testB, testErr := local.Execute(testctx, "/opt/google-cloud-sdk/bin/gcloud compute instances create instance-1 --machine-type=n1-standard-1 --zone=asia-northeast3-b --preemptible --no-restart-on-failure --maintenance-policy=terminate", false)
	//testA, testB, testErr := local.Execute(testctx, "/opt/google-cloud-sdk/bin/gcloud compute networks list", false)
    //fmt.Println(string(testB))
    //re := regexp.MustCompile(`\r?\n`)
    //testStrA := string(testA)
    //testStrA = re.ReplaceAllString(testStrA, " ") 
    //log.Infof("This is the test messge to log file")
    //fmt.Printf("***** ***** < %s > \n\n\n", testStrA)

	//// gcloud compute instances create instance-1 --machine-type=n1-standard-1 --zone=asia-northeast3-b --preemptible --no-restart-on-failure --maintenance-policy=terminate

    gcloudctx := context.Background()

    c, err := compute.NewZonesRESTClient(gcloudctx, option.WithCredentialsFile("/etc/gcp/sales-demo.json"))
	if err != nil {
		// TODO: Handle error.
	}
	defer c.Close()

	zoneReq := &computepb.ListZonesRequest{
        Project: "sales-demo-321300",
		// TODO: Fill request struct fields.
		// See https://pkg.go.dev/google.golang.org/genproto/googleapis/cloud/compute/v1#ListZonesRequest.
	}
	zoneit := c.List(gcloudctx, zoneReq)
	for {
		resp, err := zoneit.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			// TODO: Handle error.
		}
		// TODO: Use resp.
        fmt.Printf("The zone is <%#v>\n\n\n", *resp )
        fmt.Printf("The Region is <%#v>\n\n\n", *(*resp).Region )
        fmt.Printf("The kind is <%#v>\n\n\n", *(*resp).Region )
        if *(*resp).Region == "asia-northeast1" {
            fmt.Printf("The zone is <%s>\n\n\n", *(*resp).Name )
        }
	}

    instancesClient, err := compute.NewInstancesRESTClient(gcloudctx, option.WithCredentialsFile("/etc/gcp/sales-demo.json"))
    if err != nil {
        fmt.Printf("NewInstancesRESTClient: %v", err)
        return nil
    }
    defer instancesClient.Close()

    req := &computepb.ListInstancesRequest{
        Project: "sales-demo-321300",
        Zone:    "asia-northeast1-a",
    }

    it := instancesClient.List(ctx, req)
    fmt.Printf("Instances found in zone %s:\n\n\n", "asia-northeast1-a")
    for {
        instance, err := it.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            fmt.Println(err)
            return nil
        }
        fmt.Printf("- %s %s\n", *instance.Name, *instance.MachineType)
    }

	return nil
}

// Rollback implements the Task interface
func (r *Deploy) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (r *Deploy) String() string {
	return fmt.Sprintf("Echo: host=%s ", r.host)
}
