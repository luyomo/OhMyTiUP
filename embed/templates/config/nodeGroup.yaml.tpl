apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: elaticcluster
  region: us-east-1

nodeGroups:
  - name: admin
    desiredCapacity: 1
    privateNetworking: true
    labels:
      dedicated: admin

  - name: elastic
    desiredCapacity: 3
    privateNetworking: true
    instanceType: c5.2xlarge
    labels:
      dedicated: elastic
    taints:
      dedicated: elastic:NoSchedule
