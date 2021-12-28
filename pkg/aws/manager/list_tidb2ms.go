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

package manager

import (
	//	"errors"
	"context"
	"fmt"

	"github.com/fatih/color"

	"github.com/luyomo/tisample/pkg/ctxt"
	//	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	//	"github.com/luyomo/tisample/pkg/meta"
	"github.com/luyomo/tisample/pkg/executor"
	"github.com/luyomo/tisample/pkg/tui"
	"github.com/luyomo/tisample/pkg/utils"
	//	perrs "github.com/pingcap/errors"
)

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListTiDB2MSCluster(clusterName string, opt DeployOptions) error {

	var listTasks []*task.StepDisplay // tasks which are used to initialize environment

	ctx := context.WithValue(context.Background(), "clusterName", clusterName)
	ctx = context.WithValue(ctx, "clusterType", "tisample-tidb2ms")

	sexecutor, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: utils.CurrentUser()})
	if err != nil {
		return err
	}

	tableVPC := [][]string{
		// Header
		{"Component Name", "VPC ID", "CIDR", "Status"},
	}
	t1 := task.NewBuilder().ListVpc(&sexecutor, &tableVPC).
		BuildAsStep(fmt.Sprintf("  - Listing VPC"))

	listTasks = append(listTasks, t1)

	tableSubnets := [][]string{
		// Header
		{"Component Name", "Zone", "Subnet ID", "CIDR", "State", "VPC ID"},
	}
	t2 := task.NewBuilder().ListNetwork(&sexecutor, &tableSubnets).
		BuildAsStep(fmt.Sprintf("  - Listing Subnets"))

	listTasks = append(listTasks, t2)

	builder := task.NewBuilder().ParallelStep("+ Initialize target host environments", false, listTasks...)

	t := builder.Build()

	if err := t.Execute(ctxt.New(ctx, 10)); err != nil {
		return err
	}

	// insList := task.ListTiDB2Aurora{User: opt.User}
	// insList.Execute(ctxt.New(context.Background(), 1), clusterName, "tisample-tidb2ms", "aurora")
	// //fmt.Printf("The list is <%#v>", insList)

	// for _, v := range insList.ArnComponents {
	// 	clusterTable = append(clusterTable, []string{
	// 		v.ComponentType,
	// 		v.ComponentName,
	// 		v.ComponentID,
	// 		v.ImageID,
	// 		v.InstanceName,
	// 		v.KeyName,
	// 		v.State,
	// 		v.CIDR,
	// 		v.Zone,
	// 		v.Region,
	// 	})
	// }
	titleFont := color.New(color.FgRed, color.Bold)
	fmt.Printf("Cluster  Type:      %s\n", titleFont.Sprint("tisample-tidb2ms"))
	fmt.Printf("Cluster Name :      %s\n\n", titleFont.Sprint(clusterName))

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Resource Type:      %s\n", cyan.Sprint("VPC"))
	tui.PrintTable(tableVPC, true)

	fmt.Printf("\nResource Type:      %s\n", cyan.Sprint("Subnets"))
	tui.PrintTable(tableSubnets, true)
	return nil
}
