# Aurora DSQL: serverless Postgres, IAM auth only, public endpoint (no VPC/NAT),
# always-free tier of 100k DPUs + 1 GB per month.
resource "aws_dsql_cluster" "main" {
  deletion_protection_enabled = var.environment == "prod"

  tags = {
    Name = "${var.app_name}-${var.environment}"
  }
}
data "aws_region" "current" {}
