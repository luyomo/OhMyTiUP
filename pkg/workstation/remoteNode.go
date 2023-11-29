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
	"errors"
	"fmt"
	"time"

	"github.com/luyomo/OhMyTiUP/pkg/ctxt"
	"github.com/luyomo/OhMyTiUP/pkg/executor"
)

func (w *Workstation) getRemoteExecutor(targetIP string) (*ctxt.Executor, error) {
	if w.mapRemoteNodes[targetIP] == nil {
		_remoteNode, err := executor.New(executor.SSHTypeSystem, false, executor.SSHConfig{Host: targetIP, User: w.user, KeyFile: w.identityFile, Proxy: &executor.SSHConfig{Host: w.ipAddr, User: w.user, Port: 22, KeyFile: w.identityFile}}, []string{})
		if err != nil {
			return nil, err
		}

		if _remoteNode == nil {
			return nil, errors.New(fmt.Sprintf("Failed to connect remote server: %s", targetIP))
		}
		w.mapRemoteNodes[targetIP] = &_remoteNode
	}

	return w.mapRemoteNodes[targetIP], nil
}

func (w *Workstation) RunSerialCmdsOnRemoteNode(targetIP string, cmds []string, isRootUser bool) error {
	ctx := context.Background()
	_remoteExecutor, err := w.getRemoteExecutor(targetIP)
	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if _, _, err := (*_remoteExecutor).Execute(ctx, cmd, isRootUser, 60*time.Minute); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workstation) InstallMySQLBinToWorker(targetIP string) error {

	return w.RunSerialCmdsOnRemoteNode(targetIP, []string{
		"getent group mysql || groupadd -g 114 mysql",
		"id -u mysql || useradd -g mysql -u 108 mysql",
		"chown -R mysql:mysql /var/lib/mysql",
		"apt update -y",
		"apt install -y gnupg",
		"apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 467B942D3A79BD29 ",
		"wget -O /tmp/mysql-apt-config_0.8.18-1_all.deb https://dev.mysql.com/get/mysql-apt-config_0.8.18-1_all.deb",
		"apt update -y ",
		"debconf-set-selections <<< 'mysql-server-5.7 mysql-server/root_password 1234Abcd 1234Abcd'",
		"debconf-set-selections <<< 'mysql-server-5.7 mysql-server/root_password_again 1234Abcd 1234Abcd'",
		"DEBIAN_FRONTEND=noninteractive dpkg -i /tmp/mysql-apt-config_0.8.18-1_all.deb",
		"apt update -y ",
		"DEBIAN_FRONTEND=noninteractive apt install -y mysql-server",
		"rm -f /tmp/mysql-apt-config_0.8.18-1_all.deb",
	}, true)
}

func (w *Workstation) FormatDisk(targetIP, mountDir string) error {
	ctx := context.Background()

	_remoteExecutor, err := w.getRemoteExecutor(targetIP)
	if err != nil {
		return err
	}

	_, _, err = (*_remoteExecutor).Execute(ctx, fmt.Sprintf("mkdir -p %s", mountDir), true)
	if err != nil {
		return err
	}

	_, _, err = (*_remoteExecutor).Execute(ctx, "mkdir -p /opt/scripts", true)
	if err != nil {
		return err
	}

	_params := make(map[string]string)
	_params["MOUNT_DIR"] = mountDir
	if err = (*_remoteExecutor).TransferTemplate(ctx, "templates/scripts/fdisk.sh.tpl", "/opt/scripts/fdisk.sh", "0755", _params, true, 20); err != nil {
		return err
	}

	_, _, err = (*_remoteExecutor).Execute(ctx, "/opt/scripts/fdisk.sh", true)
	if err != nil {
		return err
	}

	return nil
}

func (w *Workstation) RenderTemplate2Remote(targetIP, sourceFile, targetDir string, data map[interface{}]interface{}, isRootUser bool) error {
	ctx := context.Background()

	_remoteExecutor, err := w.getRemoteExecutor(targetIP)
	if err != nil {
		return err
	}

	if err = (*_remoteExecutor).TransferTemplate(ctx, sourceFile, targetDir, "0644", data, isRootUser, 20); err != nil {
		return err
	}

	return nil
}
