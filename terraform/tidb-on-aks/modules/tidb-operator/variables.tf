variable "resource_group_location" {
  type        = string
  default     = "eastus"
  description = "Location of the resource group."
}

variable "cluster_name" {
  type        = string
  default     = "jaytest"
  description = "Cluster name ID"
}

variable "cluster_version" {
  type        = string
  default     = "v6.5.4"
  description = "TiDB Cluster version"
}

variable "resource_group_name_prefix" {
  type        = string
  default     = "rg"
  description = "Prefix of the resource group name that's combined with a random ID so name is unique in your Azure subscription."
}

variable "node_count" {
  type        = number
  description = "The initial quantity of nodes for the node pool."
  default     = 1
}

variable "k8s_version" {
  type        = string
  description = "K8S version"
  default     = "1.25.11"
}

variable "msi_id" {
  type        = string
  description = "The Managed Service Identity ID. Set this value if you're running this example using Managed Identity as the authentication method."
  default     = null
}

variable "username" {
  type        = string
  description = "The admin username for the new cluster."
  default     = "azureadmin"
}

variable "pd_count" {
  type        = number
  description = "The initial quantity of pd count."
  default     = 3
}

variable "tikv_count" {
  type        = number
  description = "The initial quantity of tikv count."
  default     = 3
}

variable "ticdc_node_count" {
  type        = number
  description = "The initial quantity of nodes for the node pool."
  default     = 3
}

variable "kube_config_file" {
  type        = string
  description = "The kube config file"
  default     = "~/"
}

variable "operator_version" {
  type        = string
  description = "The kube config file"
  default     = "v1.6.0"
}

variable "base_values" {
  type    = string
  default = ""
}
 
variable "override_values" {
  type    = string
  default = ""
}

variable "tidb_cluster_chart_version" {
  description = "tidb-cluster chart version"
  default     = "v1.6.0"
}

variable "kube_config" {
  type    = string
  description = "kube config content"
}

variable "cluster_endpoint" {
  type    = string
  description = "endpoint "
}
variable "cluster_ca_cert" {
  type    = string
  description = "endpoint "
}

variable "client_certificate" {
  type    = string
  description = "endpoint "
}


variable "client_key" {
  type    = string
  description = "endpoint "
}

