#!/bin/sh

export TOKEN=$1
echo "TOKEN: $TOKEN"

bao login -method=token token=$TOKEN

# KMS Transit
bao secrets enable transit
bao write transit/keys/terraform-backend exportable=true

# Identity Tokens need to be tested with a non-root entity
bao write identity/entity name=sample
bao write identity/entity-alias name=sample mount_accessor=$(bao read -field=accessor sys/auth/token) canonical_id=$(bao read -field=id identity/entity/name/sample)
bao write auth/token/roles/sample allowed_entity_aliases=sample

# JWT using Vault/Openbao
bao write identity/oidc/config issuer=http://localhost:8200
bao write identity/oidc/key/terraform-backend algorithm=ES256 allowed_client_ids="*"
bao write identity/oidc/role/terraform-backend-sample key=terraform-backend client_id=terraform-backend template='{"terraform-backend": {"project": "sample", "state": "prod"} }'

sleep infinity
