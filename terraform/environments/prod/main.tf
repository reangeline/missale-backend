terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
  backend "s3" {
    bucket       = "missale-tf-state-034362044245"
    key          = "prod/terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
  }
}

provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = {
      Project     = "missale"
      Environment = "prod"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  app_name = "missale"
  env      = "prod"
  # The owner creates this secret by hand (the .p8 key is not in Terraform
  # state); the wildcard covers the random suffix Secrets Manager appends.
  apple_signin_key_secret     = "missale/${local.env}/apple-signin-key"
  apple_signin_key_secret_arn = "arn:aws:secretsmanager:us-east-1:${data.aws_caller_identity.current.account_id}:secret:${local.apple_signin_key_secret}-*"
}

module "cognito" {
  source      = "../../modules/cognito"
  app_name    = local.app_name
  environment = local.env
}

module "dsql" {
  source      = "../../modules/dsql"
  app_name    = local.app_name
  environment = local.env
}

data "aws_caller_identity" "current" {}

module "content_cdn" {
  source      = "../../modules/content_cdn"
  app_name    = local.app_name
  environment = local.env
  account_id  = data.aws_caller_identity.current.account_id

  upload_origins = [for o in split(",", var.admin_origins) : trimspace(o) if trimspace(o) != ""]
}

module "lambda" {
  source                      = "../../modules/lambda"
  function_name               = "${local.app_name}-api-${local.env}"
  source_file                 = "../../../build/api.zip"
  cognito_pool_arn            = module.cognito.user_pool_arn
  dsql_cluster_arn            = module.dsql.arn
  content_bucket_arn          = module.content_cdn.bucket_arn
  apple_signin_key_secret_arn = local.apple_signin_key_secret_arn

  environment_variables = {
    ENVIRONMENT             = local.env
    COGNITO_USER_POOL_ID    = module.cognito.user_pool_id
    COGNITO_CLIENT_ID       = module.cognito.client_id
    DSQL_ENDPOINT           = module.dsql.endpoint
    OPENROUTER_API_KEY      = var.openrouter_api_key
    DAILY_DECISION_LIMIT    = var.daily_decision_limit
    FREE_DECISIONS          = var.free_decisions
    COGNITO_ADMIN_CLIENT_ID = module.cognito.admin_client_id
    CONTENT_BUCKET          = module.content_cdn.bucket
    ADMIN_ORIGINS           = var.admin_origins
    CONTENT_BASE_URL        = module.content_cdn.base_url
    APPLE_SIGNIN_KEY_SECRET = local.apple_signin_key_secret
    APPLE_SIGNIN_KEY_ID     = var.apple_signin_key_id
    APPLE_TEAM_ID           = var.apple_team_id
  }
}

module "api_gateway" {
  source      = "../../modules/api_gateway"
  api_name    = "${local.app_name}-api-${local.env}"
  lambda_arn  = module.lambda.function_arn
  lambda_name = module.lambda.function_name
}

# Browser origins of the admin page (comma-separated), for CORS.
variable "admin_origins" {
  type    = string
  default = "https://missale-admin.vercel.app"
}

variable "openrouter_api_key" {
  type      = string
  sensitive = true
}

# Lifetime Jev calls per account without a subscription: the onboarding's
# orientação uses two.
variable "free_decisions" {
  type    = string
  default = "2"
}

variable "daily_decision_limit" {
  type    = string
  default = "40"
}

# Sign in with Apple key id and team id (Apple Developer portal), for token
# revocation on account deletion (guideline 5.1.1(v)). Not secret by
# themselves; the private key lives only in Secrets Manager.
variable "apple_signin_key_id" {
  type    = string
  default = "44Y88AA2HY"
}

variable "apple_team_id" {
  type    = string
  default = "3Q524UT33T"
}

output "api_endpoint" { value = module.api_gateway.api_endpoint }
output "content_base_url" { value = module.content_cdn.base_url }
output "user_pool_id" { value = module.cognito.user_pool_id }
output "dsql_endpoint" { value = module.dsql.endpoint }
output "lambda_role_arn" { value = module.lambda.role_arn }
