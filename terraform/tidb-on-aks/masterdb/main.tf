variable "register_app_client_id" {}
variable "register_app_client_secret" {}

module "masteraks" {
    source = "../modules/aks"

    cluster_name = "jaytest001"
    k8s_version = "1.25.11"
    ticdc_node_count = 3
    kube_config_file = "/tmp/master_kubeconfig"
    cluster_version = "v7.1.1"

    register_app_client_id = "${var.register_app_client_id}"
    register_app_client_secret = "${var.register_app_client_secret}"
}

module "mastertidb" {
    # depends_on = [module.masteraks]

    source = "../modules/tidb-operator"

    cluster_name = "jaytest001"
    k8s_version = "1.25.11"
    ticdc_node_count = 3
    kube_config_file = "/tmp/master_kubeconfig"
    kube_config      = module.masteraks.kube_config
    cluster_ca_cert = module.masteraks.cluster_ca_certificate
    client_certificate = module.masteraks.client_certificate
    client_key = module.masteraks.client_key
    cluster_endpoint = module.masteraks.host
    cluster_version = "v7.1.1"

}
