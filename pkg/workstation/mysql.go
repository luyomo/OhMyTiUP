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

import (
	"context"
	"encoding/json"
	"fmt"
)

func (w *Workstation) ReadMySQLBinPos() (*[]map[string]interface{}, error) {
	return w.queryMySQL("SHOW MASTER STATUS")
}

func (w *Workstation) ReadMySQLEarliestBinPos() (*[]map[string]interface{}, error) {
	return w.queryMySQL("SHOW BINARY LOGS")
}

func (w *Workstation) queryMySQL(query string) (*[]map[string]interface{}, error) {
	stdout, _, err := (*w.executor).Execute(context.Background(), fmt.Sprintf("/opt/scripts/run_mysql_shell_query.sh mysql '%s'", query), false)
	if err != nil {
		return nil, err
	}

	var msgMapTemplate interface{}
	err = json.Unmarshal([]byte(stdout), &msgMapTemplate)
	if err != nil {
		return nil, err
	}

	var res []map[string]interface{}

	msgMap := msgMapTemplate.([]interface{})
	for _, _entry := range msgMap {
		res = append(res, _entry.(map[string]interface{}))
	}

	return &res, nil
}
