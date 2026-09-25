#!/bin/sh
# Creates an admin for the content page and adds them to the "admin" group.
# Cognito emails a temporary password; the first sign-in asks for a new one.
#   ./scripts/create-admin.sh dev pessoa@exemplo.com
set -eu
ENV=${1:?ambiente (dev|prod)}
EMAIL=${2:?e-mail}
POOL=$(terraform -chdir="terraform/environments/$ENV" output -raw user_pool_id)
aws cognito-idp admin-create-user --region us-east-1 --user-pool-id "$POOL" \
  --username "$EMAIL" \
  --user-attributes Name=email,Value="$EMAIL" Name=email_verified,Value=true \
  --desired-delivery-mediums EMAIL >/dev/null
aws cognito-idp admin-add-user-to-group --region us-east-1 --user-pool-id "$POOL" \
  --username "$EMAIL" --group-name admin
echo "admin criado: $EMAIL (senha provisória enviada por e-mail)"
