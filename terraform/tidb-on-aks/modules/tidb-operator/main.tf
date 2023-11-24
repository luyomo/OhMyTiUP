provider "helm" {
  alias = "initial"

  kubernetes {
    host                   = var.cluster_endpoint
    client_certificate     = base64decode(sensitive(var.client_certificate))
    client_key             = base64decode(sensitive(var.client_key))
    cluster_ca_certificate = base64decode(sensitive(var.cluster_ca_cert))
  }
}

provider "kubectl" {
    host                   = var.cluster_endpoint
    client_certificate     = base64decode(sensitive(var.client_certificate))
    client_key             = base64decode(sensitive(var.client_key))
    cluster_ca_certificate = base64decode(sensitive(var.cluster_ca_cert))
    load_config_file       = false
}

# fetch a raw multi-resource yaml
data "http" "source_crds" {
  url = "https://raw.githubusercontent.com/pingcap/tidb-operator/${var.operator_version}/manifests/crd.yaml"
}

# # split raw yaml into individual resources
data "kubectl_file_documents" "tidb_operator_crds" {
  content = data.http.source_crds.response_body
}

# apply each resource from the yaml one by one
resource "kubectl_manifest" "tidb_operator_serving_crds" {
  for_each   = data.kubectl_file_documents.tidb_operator_crds.manifests
  yaml_body  = each.value

  # Resolved: Terraform kubectl_manifest error: metadata.annotations: Too long: must have at most 262144 bytes
  server_side_apply = true
  wait              = true
  wait_for_rollout  = true
}


# https://developer.hashicorp.com/terraform/tutorials/kubernetes/kubernetes-provider
provider "kubernetes" {
  host                   = var.cluster_endpoint
  client_certificate     = base64decode(sensitive(var.client_certificate))
  client_key             = base64decode(sensitive(var.client_key))
  cluster_ca_certificate = base64decode(sensitive(var.cluster_ca_cert))
}

resource "kubernetes_namespace" "tidb-admin" {
  metadata {
    name = "tidb-admin"
  }
}

resource "kubernetes_namespace" "tidb-cluster" {
  metadata {
    name = "tidb-cluster"
  }
}

resource "helm_release" "tidb-operator" {
  provider   = helm.initial
  depends_on = [kubectl_manifest.tidb_operator_serving_crds, kubernetes_namespace.tidb-admin]
 
  repository = "http://charts.pingcap.org/"
  chart      = "tidb-operator"
  version    = var.operator_version
  namespace  = "tidb-admin"
  name       = "tidb-operator"
}

# resource "helm_release" "tidb-cluster" {
#   provider   = helm.initial
#   depends_on = [helm_release.tidb-operator]
#  
#   repository = "https://charts.pingcap.org/"
#   chart      = "tidb-cluster"
#   version    = var.tidb_cluster_chart_version
#   namespace  = "tidb-cluster"
#   name       = var.cluster_name
#   wait       = false
#  
#   values = [ 
#     var.base_values,
#     var.override_values
#   ]
#  
#   set {
#     name  = "pd.image"
#     value = "pingcap/pd:${var.cluster_version}"
#   }
#   set {
#     name  = "pd.replicas"
#     value = var.pd_count
#   }
#   set {
#     name  = "pd.storageClassName"
#     value = "managed-csi"
#   }
#   set {
#     name  = "pd.nodeSelector.dedicated"
#     value = "${var.cluster_name}-pd"
#   }
#   set {
#     name  = "pd.tolerations[0].key"
#     value = "dedicated"
#   }
#   set {
#     name  = "pd.tolerations[0].value"
#     value = "${var.cluster_name}-pd"
#   }
#   set {
#     name  = "pd.tolerations[0].operator"
#     value = "Equal"
#   }
#   set {
#     name  = "pd.tolerations[0].effect"
#     value = "NoSchedule"
#   }
#   set {
#     name  = "tikv.image"
#     value = "pingcap/tikv:${var.cluster_version}"
#   }
#   set {
#     name  = "tikv.replicas"
#     value = var.tikv_count
#   }
#   set {
#     name  = "tikv.storageClassName"
#     value = "managed-csi"
#   }
#   set {
#     name  = "tikv.nodeSelector.dedicated"
#     value = "${var.cluster_name}-tikv"
#   }
#   set {
#     name  = "tikv.tolerations[0].key"
#     value = "dedicated"
#   }
#   set {
#     name  = "tikv.tolerations[0].value"
#     value = "${var.cluster_name}-tikv"
#   }
#   set {
#     name  = "tikv.tolerations[0].operator"
#     value = "Equal"
#   }
#   set {
#     name  = "tikv.tolerations[0].effect"
#     value = "NoSchedule"
#   }
#   set {
#     name  = "tidb.image"
#     value = "pingcap/tidb:${var.cluster_version}"
#   }
#   set {
#     name  = "tidb.nodeSelector.dedicated"
#     value = "${var.cluster_name}-tidb"
#   }
#   set {
#     name  = "tidb.tolerations[0].key"
#     value = "dedicated"
#   }
#   set {
#     name  = "tidb.tolerations[0].value"
#     value = "${var.cluster_name}-tidb"
#   }
#   set {
#     name  = "tidb.tolerations[0].operator"
#     value = "Equal"
#   }
#   set {
#     name  = "tidb.tolerations[0].effect"
#     value = "NoSchedule"
#   }
#   # https://docs.pingcap.com/tidb-in-kubernetes/stable/configure-a-tidb-cluster
#   # tidb.service.type in (LoadBalancer / ClusterIP / NodePort)
#   set {
#     name  = "tidb.service.type"
#     value = "LoadBalancer"
#   }
# #   set {
# #     name  = "tiproxy.baseImage"
# #     value = "pingcap/tiproxy"
# #   }
# #   set {
# #     name  = "tiproxy.replicas"
# #     value = 2
# #   }
# #   set {
# #     name  = "tiproxy.version"
# #     value = "${var.cluster_version}"
# #   }
# }
# 
