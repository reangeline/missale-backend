variable "api_name" { type = string }
variable "lambda_arn" { type = string }
variable "lambda_name" { type = string }
variable "burst_limit" {
  type    = number
  default = 50
}
variable "rate_limit" {
  type    = number
  default = 20
}
