 output "master_resource_group_name" {
   value = module.slaveaks.resource_group_name
 }
 
 output "master_kubernetes_cluster_name" {
   value = module.slaveaks.kubernetes_cluster_name
 }
 
 output "master_client_certificate" {
   value     = module.slaveaks.client_certificate
   sensitive = true
 }
 
 output "master_client_key" {
   value     = module.slaveaks.client_key
   sensitive = true
 }
 
 output "master_cluster_ca_certificate" {
   value     = module.slaveaks.cluster_ca_certificate
   sensitive = true
 }
 
 output "master_cluster_password" {
   value     = module.slaveaks.cluster_password
   sensitive = true
 }
 
 output "master_cluster_username" {
   value     = module.slaveaks.cluster_username
   sensitive = true
 }
 
 output "master_host" {
   value     = module.slaveaks.host
   sensitive = true
 }
 
 output "master_kube_config" {
   value     = module.slaveaks.kube_config
   sensitive = true
 }

 output "master_kube_config_file" {
   value     = module.slaveaks.kube_config_file
 }
