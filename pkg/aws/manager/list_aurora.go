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
	"errors"
	//"fmt"
	"context"

	"github.com/luyomo/tisample/pkg/aws/ctxt"
	"github.com/luyomo/tisample/pkg/aws/spec"
	"github.com/luyomo/tisample/pkg/aws/task"
	"github.com/luyomo/tisample/pkg/meta"
	"github.com/luyomo/tisample/pkg/tui"
	perrs "github.com/pingcap/errors"
)

// Cluster represents a clsuter
// ListCluster list the clusters.
func (m *Manager) ListAuroraCluster(clusterName string, opt DeployOptions) error {
	insList := task.ListAurora{User: opt.User}
	insList.Execute(ctxt.New(context.Background(), 1), clusterName, "tisample-aurora")
	//fmt.Printf("The list is <%#v>", insList)

	clusterTable := [][]string{
		// Header
		{"Component Type", "Component Name", "Component Identifier", "Image ID", "Instance Name", "Key Name", "State", "CIDR", "Region", "Zone"},
	}
	for _, v := range insList.ArnComponents {
		clusterTable = append(clusterTable, []string{
			v.ComponentType,
			v.ComponentName,
			v.ComponentID,
			v.ImageID,
			v.InstanceName,
			v.KeyName,
			v.State,
			v.CIDR,
			v.Zone,
			v.Region,
		})
	}
	tui.PrintTable(clusterTable, true)
	return nil
}

// GetClusterList get the clusters list.
func (m *Manager) GetAuroraClusterList() ([]Cluster, error) {
	names, err := m.specManager.List()
	if err != nil {
		return nil, err
	}

	var clusters = []Cluster{}

	for _, name := range names {
		metadata, err := m.meta(name)
		if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
			!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
			return nil, perrs.Trace(err)
		}

		base := metadata.GetBaseMeta()

		clusters = append(clusters, Cluster{
			Name:       name,
			User:       base.User,
			Version:    base.Version,
			Path:       m.specManager.Path(name),
			PrivateKey: m.specManager.Path(name, "ssh", "id_rsa"),
		})
	}

	return clusters, nil
}
