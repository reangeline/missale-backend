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

module "lambda" {
  source           = "../../modules/lambda"
  function_name    = "${local.app_name}-api-${local.env}"
  source_file      = "../../../build/api.zip"
  cognito_pool_arn = module.cognito.user_pool_arn
  dsql_cluster_arn = module.dsql.arn

  environment_variables = {
    ENVIRONMENT          = local.env
    COGNITO_USER_POOL_ID = module.cognito.user_pool_id
    COGNITO_CLIENT_ID    = module.cognito.client_id
    DSQL_ENDPOINT        = module.dsql.endpoint
    OPENROUTER_API_KEY   = var.openrouter_api_key
    DAILY_DECISION_LIMIT = var.daily_decision_limit
    FREE_DECISIONS       = var.free_decisions
  }
}

module "api_gateway" {
  source      = "../../modules/api_gateway"
  api_name    = "${local.app_name}-api-${local.env}"
  lambda_arn  = module.lambda.function_arn
  lambda_name = module.lambda.function_name
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

output "api_endpoint" { value = module.api_gateway.api_endpoint }
output "dsql_endpoint" { value = module.dsql.endpoint }
output "lambda_role_arn" { value = module.lambda.role_arn }
