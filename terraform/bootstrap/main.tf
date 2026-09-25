# One-time, applied by hand: lets GitHub Actions deploy the Missale stacks
# without long-lived AWS keys. GitHub proves which repo and branch is running
# (OIDC), and the role only works for reangeline/missale-backend.
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
    key          = "bootstrap/terraform.tfstate"
    region       = "us-east-1"
    use_lockfile = true
  }
}

provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = { Project = "missale", ManagedBy = "terraform" }
  }
}

data "aws_caller_identity" "current" {}

locals {
  account = data.aws_caller_identity.current.account_id
  # GitHub's immutable subject (owner and repo IDs): survives renames, and a
  # new repo reusing the name can't assume the role.
  repo = "reangeline@23719026/missale-backend@1386524838"
}

resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

resource "aws_iam_role" "deploy" {
  name = "missale-github-deploy"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Condition = {
        StringEquals = { "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com" }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = [
            "repo:${local.repo}:ref:refs/heads/develop",
            "repo:${local.repo}:ref:refs/heads/main",
            "repo:${local.repo}:pull_request",
          ]
        }
      }
    }]
  })
}

# What `terraform apply` of environments/{dev,prod} and `make migrate` touch.
resource "aws_iam_role_policy" "deploy" {
  name = "missale-deploy"
  role = aws_iam_role.deploy.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "TerraformState"
        Effect   = "Allow"
        Action   = ["s3:ListBucket", "s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
        Resource = ["arn:aws:s3:::missale-tf-state-${local.account}", "arn:aws:s3:::missale-tf-state-${local.account}/*"]
      },
      {
        Sid      = "LambdaFunctions"
        Effect   = "Allow"
        Action   = ["lambda:*"]
        Resource = "arn:aws:lambda:us-east-1:${local.account}:function:missale-*"
      },
      {
        Sid    = "LambdaRoles"
        Effect = "Allow"
        Action = [
          "iam:GetRole", "iam:CreateRole", "iam:DeleteRole", "iam:UpdateAssumeRolePolicy", "iam:TagRole", "iam:UntagRole",
          "iam:ListRolePolicies", "iam:ListAttachedRolePolicies", "iam:ListInstanceProfilesForRole",
          "iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
          "iam:AttachRolePolicy", "iam:DetachRolePolicy", "iam:PassRole",
        ]
        Resource = "arn:aws:iam::${local.account}:role/missale-api-*"
      },
      {
        Sid      = "Logs"
        Effect   = "Allow"
        Action   = ["logs:*"]
        Resource = ["arn:aws:logs:us-east-1:${local.account}:log-group:/aws/lambda/missale-*", "arn:aws:logs:us-east-1:${local.account}:log-group:/aws/lambda/missale-*:*"]
      },
      {
        # These services don't let Terraform's calls be scoped by name.
        Sid      = "ApiCognitoDsql"
        Effect   = "Allow"
        Action   = ["apigateway:*", "cognito-idp:*", "dsql:*", "logs:DescribeLogGroups"]
        Resource = "*"
      },
    ]
  })
}

output "deploy_role_arn" { value = aws_iam_role.deploy.arn }
