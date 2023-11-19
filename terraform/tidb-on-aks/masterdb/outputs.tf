 output "master_resource_group_name" {
   value = module.masteraks.resource_group_name
 }
 
 output "master_kubernetes_cluster_name" {
   value = module.masteraks.kubernetes_cluster_name
 }
 
 output "master_client_certificate" {
   value     = module.masteraks.client_certificate
   sensitive = true
 }
 
 output "master_client_key" {
   value     = module.masteraks.client_key
   sensitive = true
 }
 
 output "master_cluster_ca_certificate" {
   value     = module.masteraks.cluster_ca_certificate
   sensitive = true
 }
 
 output "master_cluster_password" {
   value     = module.masteraks.cluster_password
   sensitive = true
 }
 
 output "master_cluster_username" {
   value     = module.masteraks.cluster_username
   sensitive = true
 }
 
 output "master_host" {
   value     = module.masteraks.host
   sensitive = true
 }
 
 output "master_kube_config" {
   value     = module.masteraks.kube_config
   sensitive = true
 }

 output "master_kube_config_file" {
   value     = module.masteraks.kube_config_file
 }
