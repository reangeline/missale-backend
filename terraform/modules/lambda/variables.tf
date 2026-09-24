variable "function_name" { type = string }
variable "source_file" { type = string }
variable "cognito_pool_arn" { type = string }
variable "dsql_cluster_arn" { type = string }
variable "environment_variables" {
  type      = map(string)
  sensitive = true
}
