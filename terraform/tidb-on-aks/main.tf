# Generate random resource group name
resource "random_pet" "rg_name" {
  prefix = var.resource_group_name_prefix
}

resource "azurerm_resource_group" "rg" {
  location = var.resource_group_location
  name     = random_pet.rg_name.id
}

resource "random_pet" "azurerm_kubernetes_cluster_name" {
  prefix = "cluster"
}

resource "random_pet" "azurerm_kubernetes_cluster_dns_prefix" {
  prefix = "dns"
}

resource "azurerm_kubernetes_cluster_node_pool" "pd" {
  name                  = "pd"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_F4s_v2"
  node_count            = 3
  node_labels           = {
    "dedicated" = "pd"
  }
  node_taints = [ "dedicated=pd:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "tidb" {
  name                  = "tidb"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_F8s_v2"
  node_count            = 2
  node_labels           = {
    "dedicated" = "tidb"
  }
  node_taints = [ "dedicated=tidb:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster_node_pool" "tikv" {
  name                  = "tikv"
  zones                 = [1, 2, 3]
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k8s.id
  vm_size               = "Standard_E8s_v4"
  node_count            = 3
  node_labels           = {
    "dedicated" = "tikv"
  }
  node_taints = [ "dedicated=tikv:NoSchedule" ]
}

resource "azurerm_kubernetes_cluster" "k8s" {
  location            = azurerm_resource_group.rg.location
  name                = random_pet.azurerm_kubernetes_cluster_name.id
  resource_group_name = azurerm_resource_group.rg.name
  dns_prefix          = random_pet.azurerm_kubernetes_cluster_dns_prefix.id

  identity {
    type = "SystemAssigned"
  }

  default_node_pool {
    name        = "agentpool"
    vm_size     = "Standard_D2_v2"
    node_count  = var.node_count
    node_labels = {
      "app" = "test"
    }
  }

  linux_profile {
    admin_username = var.username

    ssh_key {
      key_data = jsondecode(azapi_resource_action.ssh_public_key_gen.output).publicKey
    }
  }
  network_profile {
    network_plugin    = "kubenet"
    load_balancer_sku = "standard"
  }
}

# # Azure Virtual Network peering between Virtual Network A and B
# resource "azurerm_virtual_network_peering" "peer_a2b" {
#   name                         = "peer-vnet-a-with-b"
#   resource_group_name          = azurerm_resource_group.rg.name
#   virtual_network_name         = azurerm_virtual_network.vnet_a.name
#   remote_virtual_network_id    = azurerm_virtual_network.vnet_b.id
#   allow_virtual_network_access = true
# }
# # Azure Virtual Network peering between Virtual Network B and A
# resource "azurerm_virtual_network_peering" "peer_b2a" {
#   name                         = "peer-vnet-b-with-a"
#   resource_group_name          = azurerm_resource_group.rg.name
#   virtual_network_name         = azurerm_virtual_network.vnet_b.name
#   remote_virtual_network_id    = azurerm_virtual_network.vnet_a.id
#   allow_virtual_network_access = true
#   provider = azurerm.siteb
#   depends_on = [azurerm_virtual_network_peering.peer_a2b]
# }
