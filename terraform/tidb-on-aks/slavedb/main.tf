module "slaveaks" {
    source = "../modules/aks"

    cluster_name = "jaytest002"
    k8s_version = "1.26.3"
    ticdc_node_count = 0
    kube_config_file = "/tmp/slave_kubeconfig"
    cluster_version = "v6.5.4"
}

module "mastertidb" {
    # depends_on = [module.masteraks]

    source = "../modules/tidb-operator"

    cluster_name = "jaytest002"
    k8s_version = "1.26.3"
    ticdc_node_count = 0
    kube_config_file = "/tmp/slave_kubeconfig"
    kube_config      = module.slaveaks.kube_config
    cluster_ca_cert = module.slaveaks.cluster_ca_certificate
    client_certificate = module.slaveaks.client_certificate
    client_key = module.slaveaks.client_key
    cluster_endpoint = module.slaveaks.host
    cluster_version = "v6.5.4"
}
