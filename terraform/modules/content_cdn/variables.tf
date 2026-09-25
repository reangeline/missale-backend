variable "app_name" { type = string }
variable "environment" { type = string }
variable "account_id" { type = string }
variable "upload_origins" {
  type        = list(string)
  description = "Browser origins allowed to upload images (the admin page)."
}
