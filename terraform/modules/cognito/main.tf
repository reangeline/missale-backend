# One user per Apple account, username "apple_<sub>" (see internal/auth/cognito.go).
# No email sign-up, so no SES and no username_attributes.
resource "aws_cognito_user_pool" "main" {
  name                = "${var.app_name}-${var.environment}"
  deletion_protection = var.environment == "prod" ? "ACTIVE" : "INACTIVE"

  password_policy {
    minimum_length    = 12
    require_lowercase = true
    require_numbers   = true
    require_symbols   = true
    require_uppercase = true
  }

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  schema {
    name                = "email"
    attribute_data_type = "String"
    required            = false
    mutable             = true
  }
}

resource "aws_cognito_user_pool_client" "app" {
  name         = "${var.app_name}-ios-${var.environment}"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret               = false
  prevent_user_existence_errors = "ENABLED"
  enable_token_revocation       = true
  explicit_auth_flows           = ["ALLOW_ADMIN_USER_PASSWORD_AUTH", "ALLOW_REFRESH_TOKEN_AUTH"]

  # The app refreshes silently; after a year the user taps "Sign in with Apple" again.
  access_token_validity  = 1
  id_token_validity      = 1
  refresh_token_validity = 365

  token_validity_units {
    access_token  = "hours"
    id_token      = "hours"
    refresh_token = "days"
  }
}
