variable "function_name" { type = string }
variable "source_file" { type = string }
variable "cognito_pool_arn" { type = string }
variable "dsql_cluster_arn" { type = string }
variable "content_bucket_arn" { type = string }
# ARN of the Secrets Manager secret holding the Sign in with Apple private
# key (missale/<env>/apple-signin-key), for token revocation on account
# deletion (guideline 5.1.1(v)).
variable "apple_signin_key_secret_arn" { type = string }
variable "environment_variables" {
  type      = map(string)
  sensitive = true
}
