output "bucket" { value = aws_s3_bucket.content.bucket }
output "bucket_arn" { value = aws_s3_bucket.content.arn }
output "base_url" { value = "https://${aws_cloudfront_distribution.content.domain_name}/" }
