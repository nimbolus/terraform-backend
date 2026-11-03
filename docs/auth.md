# Authentication Backends

The authentication method is defined by the HTTP basic auth username, therefore multiple authentication method can be used to protect different Terraform states.

## HTTP Basic Auth

This authentication creates a hash value of provided HTTP basic auth password and state path to get the filename of the state. Therefore only the right combination of state path and password can fetch this exact state again. It's really simple to setup, no user or credential management required. The drawback is that the server can be used by everyone, who has access to the API endpoint, so it should only be used in secure or testing environments.

### Config

| Environment Variable | Type | Example | Description                                                                                |
|----------------------|------|---------|--------------------------------------------------------------------------------------------|
| AUTH_BASIC_ENABLED   | bool | `true`  | HTTP basic auth is enabled by default (checkout [JWT](#json-web-tokens) for other options) |

#### Example Terraform backend configuration

```hcl
terraform {
  backend "http" {
    address        = "https://<terraform-state-server>/state/project1/example"
    lock_address   = "https://<terraform-state-server>/state/project1/example"
    unlock_address = "https://<terraform-state-server>/state/project1/example"
    username       = "basic"
    password       = "some-random-secret"
  }
}
```

## JSON Web Tokens

JWT allow granting access to a state for a given time (the token lifetime). By default, the project and ID of the state must be part of the `terraform-backend` token claim ([other claims](#using-other-claims) can be used instead).

`terraform-backend` token claim format:

```json
{
    "terraform-backend": {
        "project": "project1",
        "state": "example"
    }
}
```

NOTE: `state` value can be set to `*` to allow accessing all project states

### Config

| Environment Variable           | Type   | Example                                         | Description                                                                                                                                                                                                    |
|--------------------------------|--------|-------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| AUTH_JWT_OIDC_ISSUER_URL       | string | `https://vault.example.com/v1/identity/oidc`    | Issuer URL which is used to validate token (if not defined, `$VAULT_ADDR/v1/identity/oidc` is used; if neither is defined, JWT auth is disabled)                                                               |
| AUTH_JWT_OIDC_CLIENT_ID        | string | `terraform-backend` (Default)                   | Client ID (string or URI) used for validating token audience claim                                                                                                                                             |
| AUTH_JWT_PROJECT_EXPRESSION    | string | `claims["terraform-backend"].project` (Default) | [CEL](https://cel.dev) expression returning the project or a list of projects the token grants access to (see below)                                                                                           |
| AUTH_JWT_STATE_EXPRESSION      | string | `claims["terraform-backend"].state` (Default)   | [CEL](https://cel.dev) expression returning the state or a list of states the token grants access to, `*` being all states (see below). If only `AUTH_JWT_PROJECT_EXPRESSION` is defined, it defaults to `"*"` |
| AUTH_JWT_VALIDATION_EXPRESSION | string | `claims.repository_owner_id == "123"`           | [CEL](https://cel.dev) expression which must return `true` for a token to be accepted (see below)                                                                                                              |

#### Example Terraform backend configuration

```hcl
terraform {
  backend "http" {
    address        = "https://<terraform-state-server>/state/project1/example"
    lock_address   = "https://<terraform-state-server>/state/project1/example"
    unlock_address = "https://<terraform-state-server>/state/project1/example"
    username       = "jwt"
    password       = "<json-web-token>"
  }
}
```

### Using other claims

`AUTH_JWT_PROJECT_EXPRESSION` and `AUTH_JWT_STATE_EXPRESSION` allow identity providers which cannot emit the `terraform-backend` claim, such as the OIDC ID tokens of GitHub or Forgejo Actions, to be used. They are [CEL](https://cel.dev) expressions over the token claims (available as `claims`), which return a string or a list of strings. A token grants access to a state if its project is one of the returned projects and its name is one of the returned states. With lists of both, this applies to every combination.

| Expression                                                            | Result                                                    |
|-----------------------------------------------------------------------|-----------------------------------------------------------|
| `claims.repository_id`                                                | Value of a string claim                                   |
| `string(claims.repository_id)`                                        | Value of a string or integer claim                        |
| `claims.repository.replace("/", "_")`                                 | `owner/repo` becomes `owner_repo`                         |
| `claims.groups.filter(g, g.startsWith("tf-")).map(g, g.substring(3))` | `tf-network` and `tf-dns` groups give `network` and `dns` |
| `claims.ref == "refs/heads/main" ? "*" : "dev"`                       | All states on the main branch, otherwise only `dev`       |

Claims with a `-` in their name have to be accessed like `claims["terraform-backend"]`. Note that `string()` also converts other types, e.g. `true` to `"true"` and `42.0` to `"42"`. The [string extension functions](https://pkg.go.dev/cel.dev/cel-go/ext#Strings) are available in addition to the standard functions.

Tokens are rejected if an expression fails (e.g. a used claim is missing) or returns anything other than a string or a list of strings. A state value of `*` grants access to all states of the project.

If only `AUTH_JWT_PROJECT_EXPRESSION` is defined, all states of the project can be accessed. If only `AUTH_JWT_STATE_EXPRESSION` is defined, the project is taken from the `terraform-backend` claim.

The used claims must be unique and stable identifiers controlled by the identity provider and not by the token requester (e.g. `repository_id`, not `sub` or a user-chosen value). As `project` and `state` are single URL path segments, values containing `/` (e.g. `owner/repo`) can never match.

### Restricting tokens

When using an identity provider, which issues tokens to anyone (e.g. GitHub Actions, where every workflow can request a token with any audience), everyone could store states in their own project. `AUTH_JWT_VALIDATION_EXPRESSION` restricts the accepted tokens with a [CEL](https://cel.dev) expression over the claims, which must return `true`. It applies to all tokens, including the ones using the `terraform-backend` claim.

```sh
# only repositories of the GitHub organization or user with the ID 123
AUTH_JWT_VALIDATION_EXPRESSION='claims.repository_owner_id == "123"'

# claims can be single values or lists, the expression decides how to match them
AUTH_JWT_VALIDATION_EXPRESSION='claims.repository_owner_id in ["123", "456"] && "terraform" in claims.groups'

# only the main branch and tags
AUTH_JWT_VALIDATION_EXPRESSION='claims.ref == "refs/heads/main" || claims.ref.startsWith("refs/tags/")'

# claims which may be missing must be checked with has()
AUTH_JWT_VALIDATION_EXPRESSION='!has(claims.act)'
```

Tokens are rejected if the expression fails, e.g. if a used claim is missing. Invalid expressions prevent the server from starting.

#### Example GitHub Actions configuration

```sh
AUTH_JWT_OIDC_ISSUER_URL=https://token.actions.githubusercontent.com
AUTH_JWT_PROJECT_EXPRESSION='claims.repository_id'
AUTH_JWT_VALIDATION_EXPRESSION='claims.repository_owner_id == "<owner id>"'
```

### Example using HashiCorp Vault Identity Tokens

HashiCorp Vault allows creating [Identity Tokens](https://www.vaultproject.io/docs/secrets/identity/identity-token) for third party systems, so that Vault policies can be used to give access to specific Terraform states.

Terraform code for creating a OIDC role for accessing the state at `https://<terraform-state-server>/state/project1/exmaple`:

```hcl
resource "vault_identity_oidc_key" "example" {
  name      = "example"
  algorithm = "RS256"
}

resource "vault_identity_oidc_role" "example" {
  name = "example"
  key  = vault_identity_oidc_key.example.name
  # token is valid for one hour
  ttl  = 3600

  template = <<-EOT
    {
      "terraform-backend": {
        "project": "project1",
        "state": "example"
      }
    }
    EOT
}

resource "vault_identity_oidc_key_allowed_client_id" "example" {
  key_name          = vault_identity_oidc_key.example.name
  allowed_client_id = vault_identity_oidc_role.example.client_id
}
```

### Terraform backend configuration

```hcl
terraform {
  backend "http" {
    address        = "https://<terraform-state-server>/state/project1/example"
    lock_address   = "https://<terraform-state-server>/state/project1/example"
    unlock_address = "https://<terraform-state-server>/state/project1/example"
    username       = "jwt"
  }
}
```

Terraform backend initialization (requires initialized HashiCorp Vault CLI client session):

```sh
TOKEN=$(vault read -field token identity/oidc/token/example)
terraform init -backend-config="password=$TOKEN" -reconfigure
```
