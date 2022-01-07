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
# Follow 官方 global 配置项设计；在官方基础上增加Region和AZ信息
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/tidb/tidb-deploy"
  data_dir: "/tidb/tidb-data"
  vpc: XXXXX # 配置全局，可选 默认 VPC 自动生成
  iam: XXXXX # 配置，可选，默认自动生成 iam 所有节点的 iam 角色
  region: # 配置所有节点的默认 region。

instance_template: # 可选，会提供几个默认的实例模板
  - micro_ins1:
      instance_type: t3.micro
      image_id: xxxx # 可选，默认值只要是一个tidb能run的镜像即可
      - storage: # 可选，设定挂几个 EBS，以及挂在那里
         type: gp3
         iops: 3000
         mount: "/tidb1"
         capacity: 256GB
      - storage:
         type: gp2
         iops: 3000
         mount: "/tidb2"
         capacity: 256GB
        
lb: # 负载均衡器配置
   enable: true 
   # TODO：可能还需要配入口 IP，路由规则等，    


# # Monitored variables are applied to all the machines.
monitored: 
# 细节略，直接 Follow 官方设计，不做修改


server_configs:
# 细节略，直接 Follow 官方设计，不做修改

pd_servers:
  - instance: micro_ins1
    # ssh_port: 22
    # name: "pd-1"
    # client_port: 2379
    # peer_port: 2380
    # deploy_dir: "/tidb-deploy/pd-2379"
    # data_dir: "/tidb-data/pd-2379"
    # log_dir: "/tidb-deploy/pd-2379/log"
    # numa_node: "0,1"
    # # The following configs are used to overwrite the `server_configs.pd` values.
    # config:
    #   schedule.max-merge-region-size: 20
    #   schedule.max-merge-region-keys: 200000 
    # 上面是官方提供的配置，我们直接follow，此外，每一个组件节点都加入一下配置
    vpc: XXXXX # 配置当前节点 VPC
    iam: XXXXX # 配置所前点的 iam 角色
    az: # 配置当前节点可用区，可选
    count: 3

tidb_servers: 
  - instance: micro_ins1
    # ssh_port: 22
    # port: 4000
    # status_port: 10080
    # deploy_dir: "/tidb-deploy/tidb-4000"
    # log_dir: "/tidb-deploy/tidb-4000/log"
    # numa_node: "0,1"
    # # The following configs are used to overwrite the `server_configs.tidb` values.
    # config:
    #   log.slow-query-file: tidb-slow-overwrited.log 
    # 上面是官方提供的配置，我们直接follow，此外，每一个组件节点都加入一下配置
    vpc: XXXXX # 配置当前节点 VPC
    iam: XXXXX # 配置所前点的 iam 角色
    az: # 配置当前节点可用区
  - instance: micro_ins1 
  - instance: micro_ins1

tikv_servers: 
  - instance: micro_ins1
    # ssh_port: 22
    # port: 20160
    # status_port: 20180
    # deploy_dir: "/tidb-deploy/tikv-20160"
    # data_dir: "/tidb-data/tikv-20160"
    # log_dir: "/tidb-deploy/tikv-20160/log"
    # numa_node: "0,1"
    # # The following configs are used to overwrite the `server_configs.tikv` values.
    # config:
    #   server.grpc-concurrency: 4
    #   server.labels: { zone: "zone1", dc: "dc1", host: "host1" }

  - instance: micro_ins1 
  - instance: micro_ins1

cdc_servers: 
  - instance: micro_ins1
    # port: 8300
    # deploy_dir: "/tidb-deploy/cdc-8300"
    # data_dir: "/tidb-data/cdc-8300"
    # log_dir: "/tidb-deploy/cdc-8300/log"
    # gc-ttl: 86400 
  - instance: micro_ins1
  - instance: micro_ins1

monitoring_servers:
  - host: micro_ins1

grafana_servers:
  - host: micro_ins1

alertmanager_servers:
  - host: micro_ins1
  
aws_cloud_formation_configs:
    template_body_file_path: xxxx.json # 和 template_url 任选其一
    template_url: https://  
    parameters:
     - param1: vvv
     - user: root
     #...

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

