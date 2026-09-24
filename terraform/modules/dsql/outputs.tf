output "arn" { value = aws_dsql_cluster.main.arn }
output "identifier" { value = aws_dsql_cluster.main.identifier }
output "endpoint" { value = "${aws_dsql_cluster.main.identifier}.dsql.${data.aws_region.current.region}.on.aws" }
