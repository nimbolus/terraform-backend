## Storage backends

The storage backend stores the state locally or remotely (depending on the implementation).

NOTE: The state path is always hashed, so getting the state name of project from the file or object name isn't possible.

## Local File System

This backend saves the state file to a local directory.

### Config
Set `STORAGE_BACKEND` to `fs`.

| Environment Variable | Type   | Default    | Description                          |
|----------------------|--------|------------|--------------------------------------|
| STORAGE_FS_DIR       | string | `./states` | Local directory to store state files |

## S3 Object Storage

The S3 backend stores the state files in any S3-compatible object store using the [MinIO SDK](https://docs.min.io/docs/golang-client-quickstart-guide.html). Since locking is handled by the Terraform backend server separately, the S3 API doesn't need support for write-once-read-many (WORM).

### Config
Set `STORAGE_BACKEND` to `fs`.

| Environment Variable  | Type   | Default            | Description             |
|-----------------------|--------|--------------------|-------------------------|
| STORAGE_S3_ENDPOINT   | string | `s3.amazonaws.com` | S3 endpoint             |
| STORAGE_S3_USE_SSL    | string | `true`             | Use SSL for S3 endpoint |
| STORAGE_S3_ACCESS_KEY | string | --                 | S3 Access key ID        |
| STORAGE_S3_SECRET_KEY | string | --                 | S3 Secret key           |
| STORAGE_S3_BUCKET     | string | `terraform-state`  | Name of the S3 bucket   |


## Postgres

The Postgres backend stores state files in a database table.

### Config
Set `STORAGE_BACKEND` to `postgres`.

| Environment Variable   | Type   | Default  | Description                            |
|------------------------|--------|----------|----------------------------------------|
| STORAGE_POSTGRES_TABLE | string | `states` | The table name used for storing states |

Make sure that the [Postgres client](clients.md#postgres-client) is set up properly.
