
data "azurerm_resource_group" "rg" {
  name = "${var.resource_group}"
}

resource "azurerm_kubernetes_cluster_node_pool" "pd" {
  name                  = "pd"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_F4s_v2"
  node_count            = 3
  node_labels           = {
    "dedicated" = "${var.cluster_name}-pd"
  }
  node_taints = [ "dedicated=${var.cluster_name}-pd:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "tidb" {
  name                  = "tidb"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_F8s_v2"
  node_count            = 2
  node_labels           = {
    "dedicated" = "${var.cluster_name}-tidb"
  }
  node_taints = [ "dedicated=${var.cluster_name}-tidb:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "tikv" {
  name                  = "tikv"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_E8s_v4"
  node_count            = 3
  node_labels           = {
    "dedicated" = "${var.cluster_name}-tikv"
  }
  node_taints = [ "dedicated=${var.cluster_name}-tikv:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "tiflash" {
  count = var.tiflash_node_count == 0 ? 0 : 1

  name                  = "tiflash"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_E8s_v4"
  node_count            = 3
  node_labels           = {
    "dedicated" = "${var.cluster_name}-tiflash"
  }
  node_taints = [ "dedicated=${var.cluster_name}-tiflash:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "lightning" {
  name                  = "lightning"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_E8s_v4"
  node_count            = 1
  node_labels           = {
    "dedicated" = "${var.cluster_name}-lightning"
  }
  node_taints = [ "dedicated=${var.cluster_name}-lightning:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "ticdc" {
  count = var.ticdc_node_count == 0 ? 0 : 1

  name                  = "ticdc"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_F8s_v2"
  node_count            = var.ticdc_node_count
  node_labels           = {
    "dedicated" = "${var.cluster_name}-ticdc"
  }
  node_taints = [ "dedicated=${var.cluster_name}-ticdc:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster" "k8s" {
  location            = var.resource_group_location
  name                = var.cluster_name
  resource_group_name = data.azurerm_resource_group.rg.name
  dns_prefix          = var.cluster_name
  kubernetes_version  = var.k8s_version

  service_principal {
    client_id     = var.register_app_client_id
    client_secret = var.register_app_client_secret
  }

  default_node_pool {
    name        = "agentpool"
    vm_size     = "Standard_D2_v2"
    node_count  = var.node_count
    node_labels = {
      "app" = "jay"
    }
  }

  linux_profile {
    admin_username = var.username

    ssh_key {
      key_data = azapi_resource_action.ssh_public_key_gen.output.publicKey
    }
  }
  network_profile {
    network_plugin    = "kubenet"
    load_balancer_sku = "standard"
  }
}

resource "local_sensitive_file" "kubeconfig" {
  depends_on        = [azurerm_kubernetes_cluster.k8s]
  content           = azurerm_kubernetes_cluster.k8s.kube_config_raw
  filename          = var.kube_config_file
}

