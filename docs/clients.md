# Clients

## Vault Client

Requests against a Vault server are handled by this client. Since most requests need authentication, a Vault token can be defined in the environment or fetched by the client for example with a Kubernetes service account.

**Config**
| Environment Variable | Type   | Default      | Description                                                                                                                    |
|----------------------|--------|--------------|--------------------------------------------------------------------------------------------------------------------------------|
| VAULT_ADDR           | string | --           | see [Vault Environment Variables](https://www.vaultproject.io/docs/commands#environment-variables)                             |
| VAULT_TOKEN          | string | --           | see [Vault Environment Variables](https://www.vaultproject.io/docs/commands#environment-variables)                             |
| VAULT_KUBE_AUTH_NAME | string | `kubernetes` | Name of the Kubernetes auth backend mount point, see [Vault Kubernetes Auth](https://www.vaultproject.io/docs/auth/kubernetes) |
| VAULT_KUBE_AUTH_ROLE | string | --           | Name of the Kubernetes auth backend role, see [Vault Kubernetes Auth](https://www.vaultproject.io/docs/auth/kubernetes)        |

## Redis Client

This client handles Redis requests.

**Config**
| Environment Variable | Type   | Default          | Description                             |
|----------------------|--------|------------------|-----------------------------------------|
| REDIS_ADDR           | string | `localhost:6379` | Host and port of the Redis instance     |
| REDIS_PASSWORD       | string | --               | An optional password for authentication |
