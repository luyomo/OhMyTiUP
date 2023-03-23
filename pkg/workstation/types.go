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

package workstation

import ()

type RedshiftDBInfo struct {
	Host     string `yaml:"host"`
	Port     int32  `yaml:"port"`
	DBName   string `yaml:"db_name"`
	UserName string `yaml:"user_name"`
	Password string `yaml:"password"`
	Status   string
	NodeType string
}
