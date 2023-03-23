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
	// "errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"strings"

	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

/*
   1. Read mapping file
   2. Input: Data type, mapping file
   3. Output: Query01, Query02, InsertQuery
   4. Func:
   4.1 generateSourceDDL()
   4.2 generateTargetDDL()
   4.3 generateInsertQuery()
   4.4 CreateSourceTable()
   4.5 CreateTargetTable()
   4.6 GenerateInsertFile()

   SourceScript: /opt/scripts/run_tidb_query
   SourceDB: test

   TargetScript: /opt/scripts/run_redshift_query
   TargetDB: dev
*/

/******************************************************************************/
func (b *Builder) CreatePerfTables(wsExe *ctxt.Executor, mappingFile string, colTypesList []string) *Builder {
	b.tasks = append(b.tasks, &PerfTables{wsExe: wsExe, mappingFile: mappingFile, colTypesList: colTypesList})
	return b
}

/******************************************************************************/

type MetaTable struct {
	DBType      string   `yaml:"DBType"`
	DBName      string   `yaml:"DBName"`
	PK          string   `yaml:"PK"`
	Executor    string   `yaml:"Executor"`
	TailColumns []string `yaml:"TailColumns"`
}

type ColumnDef struct {
	DataType string   `yaml:"DataType"`
	Def      string   `yaml:"Def"`
	Queries  []string `yaml:"Query,omitempty"`
}

type ColumnMapping struct {
	Meta struct {
		TableName string    `yaml:"TableName"`
		Source    MetaTable `yaml:"Source"`
		Target    MetaTable `yaml:"Target"`
	} `yaml:"Meta"`
	Mapping []struct {
		Source ColumnDef `yaml:"Source"`
		Target ColumnDef `yaml:"Target"`
		Value  string    `yaml:"Value"`
	} `yaml:"Mapping"`
}

type PerfTables struct {
	wsExe       *ctxt.Executor
	mappingFile string

	colTypesList  []string
	columnMapping ColumnMapping
}

// Execute implements the Task interface
func (c *PerfTables) Execute(ctx context.Context) error {

	log.Infof("***** PerfTables ****** \n\n\n")

	mapFile, err := ioutil.ReadFile(c.mappingFile)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(mapFile, &c.columnMapping)
	if err != nil {
		return err
	}

	if err := c.generateInsertFile(); err != nil {
		return err
	}

	if err := c.createTable("source"); err != nil {
		return err
	}

	if err := c.createTable("target"); err != nil {
		return err
	}

	return nil
}

// Rollback implements the Task interface
func (c *PerfTables) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *PerfTables) String() string {
	return fmt.Sprintf("Echo: Create Perf tables  ... ...  ")
}

func (c *PerfTables) dropTableIfExist(sourceTargetType string) error {
	// 1. Check the table existness
	var metaTable MetaTable

	ctx := context.Background()

	if sourceTargetType == "source" {
		metaTable = c.columnMapping.Meta.Source
	} else {
		metaTable = c.columnMapping.Meta.Target
	}

	/*
	   First check the db connection's status, if the connection is ok, try to access the table. It returns error if the table does not exist. otherwise drop the table.
	*/
	if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("%s %s 'select 1'", metaTable.Executor, metaTable.DBName), false); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("%s %s 'select count(*) from %s'", metaTable.Executor, metaTable.DBName, c.columnMapping.Meta.TableName), false); err != nil {
		return nil
	}

	if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("%s %s 'drop table %s'", metaTable.Executor, metaTable.DBName, c.columnMapping.Meta.TableName), false); err != nil {
		return err
	}

	return nil
}

func (c *PerfTables) generateDDL(sourceTargetType string) string {

	var arrTblColDef []string // Array to keep column definition. ex: ["pk_col BIGINT PRIMARY KEY AUTO_RANDOM", ... , "tidb_timestamp timestamp default current_timestamp"]

	var metaTable MetaTable
	var columnDef ColumnDef

	if sourceTargetType == "source" {
		metaTable = c.columnMapping.Meta.Source
	} else {
		metaTable = c.columnMapping.Meta.Target
	}

	arrTblColDef = append(arrTblColDef, metaTable.PK)

	for _, _dataType := range c.colTypesList {
		for _, _mapItem := range c.columnMapping.Mapping {
			if sourceTargetType == "source" {
				columnDef = _mapItem.Source
			} else {
				columnDef = _mapItem.Target
			}

			// ColumnDef
			if _dataType == _mapItem.Source.DataType {
				arrTblColDef = append(arrTblColDef, columnDef.Def)
			}
		}
	}
	arrTblColDef = append(arrTblColDef, metaTable.TailColumns...)

	return fmt.Sprintf("create table %s (%s)", c.columnMapping.Meta.TableName, strings.Join(arrTblColDef, ", "))
}

func (c *PerfTables) generateInsertFile() error {
	var arrCols []string
	var arrData []string

	var columnDef ColumnDef

	ctx := context.Background()

	for _, _dataType := range c.colTypesList {
		for _, _mapItem := range c.columnMapping.Mapping {
			columnDef = _mapItem.Source

			// ColumnDef
			if _dataType == columnDef.DataType {
				arrCols = append(arrCols, strings.Split(_mapItem.Source.Def, " ")[0])
				arrData = append(arrData, strings.Replace(strings.Replace(_mapItem.Value, "<<<<", "'", 1), ">>>>", "'", 1))
			}
		}
	}

	strInsQuery := fmt.Sprintf("insert into %s(%s) values(%s)", c.columnMapping.Meta.TableName, strings.Join(arrCols, ","), strings.Join(arrData, ","))
	if _, _, err := (*c.wsExe).Execute(ctx, fmt.Sprintf("echo \\\"%s\\\" > /tmp/query.sql", strInsQuery), true); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, "mv /tmp/query.sql /opt/kafka/", true); err != nil {
		return err
	}

	if _, _, err := (*c.wsExe).Execute(ctx, "chmod 777 /opt/kafka/query.sql", true); err != nil {
		return err
	}

	return nil
}

func (c *PerfTables) createTable(sourceTargetType string) error {
	// Drop the table if it exists
	if err := c.dropTableIfExist(sourceTargetType); err != nil {
		return err
	}

	// Generate the ddl create statement
	createTableQuery := c.generateDDL(sourceTargetType)

	// Create the table
	var metaTable MetaTable
	if sourceTargetType == "source" {
		metaTable = c.columnMapping.Meta.Source
	} else {
		metaTable = c.columnMapping.Meta.Target
	}

	if _, _, err := (*c.wsExe).Execute(context.Background(), fmt.Sprintf("%s %s '%s'", metaTable.Executor, metaTable.DBName, createTableQuery), false); err != nil {
		return err
	}

	return nil
}
