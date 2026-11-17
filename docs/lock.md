# Lock Backends

The lock backend takes care of locking a specific state file, so that only one Terraform entity can access and change it in a given time.

## Local Map

This is the simplest implementation by using a local Golang map and doesn't require any configuration. It works fine for a standalone, single-instance Terraform backend server, but doesn't scale. Also if the Terraform backend server crashes, the lock information will be lost.

### Config

Set `LOCK_BACKEND` to `local`.

## Local Files

This backend keeps each lock in a file in a local directory, so that a lock stays when the Terraform backend server restarts or crashes. A lock file is replaced atomically. It doesn't require an external service, but like the local map it only works for a standalone, single-instance Terraform backend server: the mutex is in the process, so two servers on the same directory can both get a lock.

### Config

Set `LOCK_BACKEND` to `file`.

| Environment Variable | Type   | Default   | Description                                 |
|----------------------|--------|-----------|---------------------------------------------|
| LOCK_FILE_DIR        | string | `./locks` | The directory used for storing lock files   |

## Redis

This backend uses an external Redis server to lock the states. It's scalable and can be used also with multiple Terraform backend server instances.

### Config

Set `LOCK_BACKEND` to `redis`.

Make sure that the [Redis client](clients.md#redis-client) is set up properly.

## Postgres

This backend uses an external Postgres server to lock the states. It's scalable and can be used also with multiple Terraform backend server instances.

### Config

Set `LOCK_BACKEND` to `postgres`.

| Environment Variable | Type   | Default | Description                           |
|----------------------|--------|---------|---------------------------------------|
| LOCK_POSTGRES_TABLE  | string | `locks` | The table name used for storing locks |

Make sure that the [Postgres client](clients.md#postgres-client) is set up properly.
