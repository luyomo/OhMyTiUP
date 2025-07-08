variable "register_app_client_id" {}
variable "register_app_client_secret" {}

module "masteraks" {
    source = "../modules/aks"

    cluster_name = "tidb-on-aks-demo"                                  # Cluster Name
    k8s_version = "1.30.0"                                             # K8S version
#    ticdc_node_count = 3                                              # Number TiCDC nodes
    kube_config_file = "/tmp/master_kubeconfig"                        # K8S Config file 
    cluster_version = "v7.5.2"                                         # TiDB Version

    register_app_client_id = "${var.register_app_client_id}"           # Azure APP Registration client id
    register_app_client_secret = "${var.register_app_client_secret}"   # Azure APP Registration secret id
    resource_group = "jp-presale-test"                                 # resource group name in which AKS is created
}

module "mastertidb" {
    # depends_on = [module.masteraks]

    source = "../modules/tidb-operator"

    cluster_name = "tidb-on-aks-demo"                                  # Cluster Name 
    k8s_version = "1.30.0"                                             # K8S version
#    ticdc_node_count = 3                                              # Number of TiCDC nodes
    kube_config_file = "/tmp/master_kubeconfig"
    cluster_version = "v7.5.2"                                         # TiDB Version
    kube_config      = module.masteraks.kube_config
    cluster_ca_cert = module.masteraks.cluster_ca_certificate
    client_certificate = module.masteraks.client_certificate
    client_key = module.masteraks.client_key
    cluster_endpoint = module.masteraks.host

}
