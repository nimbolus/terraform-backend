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
LOG_LEVEL=debug go run cmd/terraform-backend.go
```

or use [docker-compose](./docker-compose.yml):

```sh
docker-compose up -d
```

### Default settings

The following table describes the default configuration, although the backend server will run with these values, it's not scalable and therefore only for testing purposes.

| Environment Variable | Type   | Default     | Description                                                                                                          |
|----------------------|--------|-------------|----------------------------------------------------------------------------------------------------------------------|
| LOG_LEVEL            | string | `info`      | Log level (options are: `fatal`, `info`, `warning`, `debug`, `trace`)                                                |
| LISTEN_ADDR          | string | `:8080`     | Address the HTTP server listens on                                                                                   |
| TLS_KEY              | string | --          | Path to TLS key file for listening with TLS (fallback to HTTP if not specified)                                      |
| TLS_CERT             | string | --          | Path to TLS certificate file for listening with TLS (fallback to HTTP if not specified)                              |
| MAX_BODY_SIZE        | int    | `104857600` | Maximum request body size in bytes (100 MiB), larger requests are rejected with `413`                                |
| STORAGE_BACKEND      | string | `fs`        | Module for state file storage (checkout [docs/storage.md](./docs/storage.md) for other options)                      |
| STORAGE_FS_DIR       | string | `./states`  | File system directory for `fs` storage module to store state files                                                   |
| KMS_BACKEND          | string | `local`     | Module used for encryption (checkout [docs/kms.md](./docs/kms.md) for other options)                                 |
| KMS_KEY              | string | --          | Key for `local` KMS module. If neither this nor KMS_KEY_FILE is defined, the server will generate a new one and exit |
| KMS_KEY_FILE         | string | --          | file containing the value for KMS_KEY, will take precedence.                                                         |
| LOCK_BACKEND         | string | `local`     | Module used for locking the state (checkout [docs/lock.md](./docs/lock.md) for other options)                        |
| AUTH_BASIC_ENABLED   | bool   | `true`      | HTTP basic auth is enabled by default (checkout [docs/auth.md](./docs/auth.md) for other options)                    |
| FORCE_UNLOCK_ENABLED | bool   | `true`      | Force-unlock feature enables the native Terraform behavior which unlocks the state even if no lock id was sent       |

## Usage

The path to the state is: `/state/<project-id>/<state-name>`.

### Example Terraform backend configuration

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
docker-compose up -d --wait redis postgres minio openbao openbao-init
INTEGRATION_TEST=true go test ./... -count=1
```

## Releasing

Pushing a `v*` tag runs the release workflow: it runs the tests, checks that [CHANGELOG.md](./CHANGELOG.md) is up to date, creates a draft release with the changelog section of the version as notes, publishes the container image and then the release.

The changelog check regenerates `CHANGELOG.md` from the tag and compares it with the committed one. The version heading takes its date and compare link from the annotated tag. `--tag-unreleased` renders the unreleased commits as if `HEAD` was already tagged, so the changelog is generated before the tag and the tag goes on the release commit:

```sh
v=0.3.0
convco changelog --ignore-prereleases --unreleased "$v" --tag-unreleased --output CHANGELOG.md
git add CHANGELOG.md && git commit -m "chore: release v$v"
git tag -a "v$v" -m "v$v"
git push origin main "v$v"
```

- `--tag-unreleased` was added in [convco#457](https://github.com/convco/convco/pull/457), but isn't in a convco release yet, so it needs a build from source. CI does not need it, it generates the changelog from the pushed tag.
- `--unreleased` takes the version without the `v` prefix.
- The tag has to be annotated and created the same day, a lightweight tag gets the commit date.
- `chore:` commits are not listed in the changelog, so the release commit does not change it.
- Commits with a breaking change need a `BREAKING CHANGE:` footer. `!` alone bumps the version, but doesn't add a note to the changelog.

Release candidates (tags with a `-`, like `v0.3.0-rc.1`) are published as prereleases and don't update the `latest` image. They are not added to `CHANGELOG.md` and need no release commit, the tag is pushed directly. The changelog check is skipped for them, their release notes cover the commits since the previous tag.
