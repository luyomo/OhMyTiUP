[![LICENSE](https://img.shields.io/github/license/pingcap/tidb.svg)](https://github.com/pingcap/tiup/blob/master/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/pingcap/tiup)](https://goreportcard.com/badge/github.com/pingcap/tiup)
[![Coverage Status](https://codecov.io/gh/pingcap/tiup/branch/master/graph/badge.svg)](https://codecov.io/gh/pingcap/tiup/)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fpingcap%2Ftiup.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fpingcap%2Ftiup?ref=badge_shield)

# Oh My TiUP

## 项目介绍

Oh My TiUP 的命名灵感来自于 [Oh My Zsh](https://ohmyz.sh/) 项目，希望能像 Oh My Zsh 一样成为一个趁手的生产力工具。Oh My TiUP 将基于 TiUP，围绕着易用性打造一系列甜品级的特性。 

## 背景&动机
TiUP 作为 TiDB 系列所有产品线的入口，是非常重要的生态工具，可谓是 TiDB 的门神。目前 TiUP 作为部署工具已经收到了广泛地认可，但仍然在一些场景存在力不从心的情况。我们将以提高易用性，降低用户门槛为宗旨，为 TiUP 支持一系列甜品级的特性。

## 项目设计

### TiUP Cluster on Cloud

#### 背景
TiUP Cluster 用于在生产上部署 TiDB 集群，提供了非常丰富的功能以适用于生产上复杂的部署环境。但 TiUP Cluster 主要是为 on-premise 场景部署而设计的，并未考虑云上场景。虽然在云上场景中，tidb operator 已经能提供丰富的集群部署和管理能力，但 tidb operator 依赖 K8s，在追求轻量和易运维的场景中并不适用。目前在云上但不使用 K8s 的场景中，TiDB 集群的部署和管理仍然是一块空白。

#### 产品设计
我们设计了一个新的名为 cloud-cluster 的 TiUP 组件，专门用于 TiDB 集群的云上部署和管理。

##### 部署集群
在部署集群时，cloud-cluster 不需要用户指定要部署的目标主机 IP，而是用户指定要部署的目标主机机型，Region、AZ 等信息，cloud-cluster 可以自动向 AWS 申请虚机并部署 TiDB 集群。

###### 能力
- deploy 集群时可以自动根据用户的配置向 AWS 申请 EC2（包含 VPC，EBS 配置）
- EC2 创建并启动完毕后，tiup 可以自动将 TiDB 产品线族部署到目标 EC2
- 根据 ec2 的 region 和 az，自动设置实例的物理位置 label
- tiup 可以自动创建 ELB，作为 TiDB 上层的负载均衡器

###### User Interface

官方 deploy 组件部署集群的命令如下：
tiup cluster deploy tidb-test v5.3.0 ./topology.yaml --user root [-p] [-i /home/root/.ssh/gcp_rsa]

cloud-deploy 组件部署集群命令如下：
tiup cloud-cluster deploy tidb-test v5.3.0 ./topology.yaml #TODO：加一些用于 AWS 授权的参数

相比与官方 deploy 指令，cloud-cluster 的 deploy 指令去除了 -p -i 这种用于 ssh 授权的指令，而添加一些用于 AWS 授权的指令

配置文件：

```
aws_topo_configs:
  general:
    imageid: ami-0ac97798ccf296e02            # Image ID for TiDB cluster's EC2 node
    keyname: jay.pingcap                      # key name to login from workstation to EC2 nodes 
    cidr: 172.83.0.0/16                       # VPC cidr
    instance_type: m5.2xlarge                 # default instance type for EC2 nodes
    tidb_version: v5.2.0                      # TiDB version to deploy
  pd:
    instance_type: m5.2xlarge                 # PD instance type
    count: 3                                  # Number of PD nodes to generate
  tidb:
    instance_type: m5.2xlarge                 # TiDB instance type
    count: 2                                  # Number of TiDB nodes to generate
  tikv:
    instance_type: m5.2xlarge                 # TiKV instance type
    count: 3                                  # Number of TiKV nodes to generate
    volumeSize: 80                            # Volume Size of the TiKV nodes
  dm:
    instance_type: t2.micro                   # DM instance type
    count: 1                                  # Number of DM node to generate
  ticdc:
    instance_type: m5.2xlarge                 # TiCDC instance type
    count: 1                                  # Number of TiCDC nodes to generate
```

##### 扩缩容集群

###### 能力
- 扩容集群时可以自动向 AWS 申请 EC2，并自动完成部署
- 缩容时可以自动停止或销毁 EC2，由用户选择是否直接销毁 EC2

###### User Interface
官方扩缩容设计见：使用 TiUP 扩容缩容 TiDB 集群
扩容配置文件（scale-out.yaml）：

```
tidb_servers:
  - instance: micro_ins1
   # ssh_port: 22
    # port: 4000
    # status_port: 10080
    # deploy_dir: "/tidb-deploy/tidb-4000"
    # log_dir: "/tidb-deploy/tidb-4000/log"
    # numa_node: "0,1"
   vpc: xxx
   az: xxx
tikv_servers: 
  - instance: micro_ins1
  - instance: micro_ins1
pd_servers: 
  - instance: micro_ins1
  - instance: micro_ins1
```


命令参数直接 follow 官方设计：
tiup cloud-cluster scale-out <cluster-name> scale-out.yaml

缩容：
tiup cloud-cluster scale-in <cluster-name> --node 10.0.1.5:20160 [--retain]

缩容指定节点
--retain 停止 EC2 而非销毁 EC2，以保留数据


##### 销毁集群
###### 能力
- 自动停止或销毁集群所使用的所有 EC2，由用户选择是否直接销毁 EC2
###### User Interface
 
```
➜  ~ tiup cloud-cluster destroy --help
Usage:
  tiup-cluster destroy <cluster-name> [flags]

Flags:
      --force                          Force will ignore remote error while destroy the cluster
  -h, --help                           help for destroy
      --retain-node-data stringArray   Specify the nodes or hosts whose data will be retained，指定需要保留的节点（停止 EC2 而非销毁 EC2）
      --retain-role-data stringArray   Specify the roles whose data will be retained，指定需要保留的节点（停止 EC2 而非销毁 EC2）
```

#### Example
##### Scaling
```
pi@ohmytiup:~/workspace/tisample $ ./bin/aws tidb2ms scale hackathon ~/workspace/hackathon/aws-tidb-simple.yaml 
Please confirm your topology:
AWS Region:      Tokyo
Cluster type:    tidb
Cluster name:    hackathon
Cluster version: v5.1.0
User Name:       admin
Key Name:        jay

Component    # of nodes  Instance Type  Image Name             CIDR           User
---------    ----------  -------------  ----------             ----           ----
Workstation  1           m5.2xlarge     ami-0ac97798ccf296e02  172.82.0.0/16  admin
TiDB         2           m5.2xlarge     ami-0ac97798ccf296e02  172.83.0.0/16  master
PD           3           m5.2xlarge     ami-0ac97798ccf296e02  172.83.0.0/16  master
TiKV         3           m5.2xlarge     ami-0ac97798ccf296e02  172.83.0.0/16  master
TiCDC        1           m5.2xlarge     ami-0ac97798ccf296e02  172.83.0.0/16  master
DM           1           t2.micro       ami-0ac97798ccf296e02  172.83.0.0/16  master
Attention:
    1. If the topology is not what you expected, check your yaml file.
    2. Please confirm there is no port/directory conflicts in same host.
Do you want to continue? [y/N]: (default=N) y
  - Preparing workstation ... Done
  - Preparing tidb servers ... Done
+ Initialize target host environments
  - Prepare Ec2  resources :22 ... Done
Cluster `hackathon` scaled successfully 
```
#### Reference
[youtube](https://www.youtube.com/watch?v=2P9Dqkaay2A&t=103s)
