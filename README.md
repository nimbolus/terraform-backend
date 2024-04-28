# Terraform Backend Server

A state backend server which implements the Terraform HTTP backend API with plugable modules for authentication, storage, locking and state encryption.

> :warning: **Disclaimer**: This code is in an early development state and not tested extensively for bugs and security issues. If you find some, please raise an issue or merge request.

Supported authentication methods:

- HTTP basic auth
- JSON Web Tokens

Supported storage backends:

- local file system
- S3
- Postgres

Supported lock backends:

- local map
- Redis
- Postgres

Supported KMS (encryption) backends:

- local AES key
- AES from HashiCorp Vault Key/Value store (v2)
- HashiCorp Vault Transit engine

## Deployment

Run locally for development:

```sh
LOG_LEVEL=debug go run ./cmd/terraform-backend
```

or use [docker-compose](./docker-compose.yml):

```sh
docker-compose up -d
```

### Default settings

The following table describes the default configuration, although the backend server will run with these values, it's not scalable and therefore only for testing purposes.

| Environment Variable | Type   | Default    | Description                                                                                       |
| -------------------- | ------ | ---------- | ------------------------------------------------------------------------------------------------- |
| LOG_LEVEL            | string | `info`     | Log level (options are: `fatal`, `info`, `warning`, `debug`, `trace`)                             |
| LISTEN_ADDR          | string | `:8080`    | Address the HTTP server listens on                                                                |
| TLS_KEY              | string | --         | Path to TLS key file for listening with TLS (fallback to HTTP if not specified)                   |
| TLS_CERT             | string | --         | Path to TLS certificate file for listening with TLS (fallback to HTTP if not specified)           |
| STORAGE_BACKEND      | string | `fs`       | Module for state file storage (checkout [docs/storage.md](./docs/storage.md) for other options)   |
| STORAGE_FS_DIR       | string | `./states` | File system directory for `fs` storage module to store state files                                |
| KMS_BACKEND          | string | `local`    | Module used for encryption (checkout [docs/kms.md](./docs/kms.md) for other options)              |
| KMS_KEY              | string | --         | Key for `local` KMS module, if not defined, the server will generate a new one and exit           |
| LOCK_BACKEND         | string | `local`    | Module used for locking the state (checkout [docs/lock.md](./docs/lock.md) for other options)     |
| AUTH_BASIC_ENABLED   | bool   | `true`     | HTTP basic auth is enabled by default (checkout [docs/auth.md](./docs/auth.md) for other options) |

## Usage

The path to the state is: `/state/<project-id>/<state-name>`.

**Example Terraform backend configuration**

```hcl
terraform {
  backend "http" {
    address        = "http://localhost:8080/state/project1/example"
    lock_address   = "http://localhost:8080/state/project1/example"
    unlock_address = "http://localhost:8080/state/project1/example"
    username       = "basic"
    password       = "some-random-secret"
  }
}
```

For more information about username and password checkout [docs/auth.md](./docs/auth.md)

## Tests

Run unit tests:

```sh
go test ./...
```

Run integration tests:

```sh
docker-compose up -d redis postgres minio
go test ./... --tags integration -count=1
```

## Speculative Runs in GitHub Actions

This project includes a CLI to trigger speculative runs via GitHub Actions, similar to how Terraform Cloud works.

### Install

To install the binary on your system, download the binary from the latest release and make it executable:

```sh
curl -L "https://github.com/ffddorf/terraform-backend/releases/latest/download/tf-preview-gh_$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)" > tf-preview-gh
sudo mv tf-preview-gh /usr/local/bin/tf-preview-gh
sudo chmod +x /usr/local/bin/tf-preview-gh
```

### Usage

Run the CLI in the directory for which you want to run a remote plan.

The tool will pick its context from the environment:

- Address of the Terraform Backend from your backend config
- Repository to use from the git remote called `origin`

```
tf-preview-gh
```
