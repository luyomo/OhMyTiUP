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

package iam

import (
	"context"
	// "encoding/json"
	"errors"
	"fmt"
	// "os"
	// "path"
	// "sort"
	// "strings"
	// "text/template"
	// "time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
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

type IAMAPI struct {
	client *iam.Client

	mapArgs *map[string]string
}

func NewIAMAPI(mapArgs *map[string]string) (*IAMAPI, error) {
	iamapi := IAMAPI{}

	if mapArgs != nil {
		iamapi.mapArgs = mapArgs
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	iamapi.client = iam.NewFromConfig(cfg)

	return &iamapi, nil
}

func (b *IAMAPI) GetRole(pathPrefix, roleName string) (*[]types.Role, error) {
	resp, err := b.client.ListRoles(context.TODO(), &iam.ListRolesInput{
		PathPrefix: aws.String(fmt.Sprintf("/%s/", pathPrefix)),
	})
	if err != nil {
		return nil, err
	}

	resRoles := []types.Role{}
	for _, role := range resp.Roles {
		if *role.RoleName == roleName {
			resRoles = append(resRoles, role)
		}
	}
	switch len(resRoles) {
	case 0:
		return nil, nil
	case 1:
		return &resRoles, nil
	default:
		return nil, errors.New("Multiple roles matched.")
	}

}

func (c *IAMAPI) makeTags() *[]types.Tag {
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

func MakeRoleName(clusterName, subClusterType string) string {
	return fmt.Sprintf("%s.%s", clusterName, subClusterType)
}
