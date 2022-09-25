# Lock Backends

The lock backend takes care of locking a specific state file, so that only one Terraform entity can access and change it in a given time.

## Local Map

This is the simplest implementation by using a local Golang map and doesn't require any configuration. It works fine for a standalone, single-instance Terraform backend server, but doesn't scale. Also if the Terraform backend server crashes, the lock information will be lost.

### Config
Set `LOCK_BACKEND` to `local`.

## Redis

This backend uses an external Redis server to lock the states. It's scalable and can be used also with multiple Terraform backend server instances.

### Config
Set `LOCK_BACKEND` to `redis`.

Make sure that the [Redis client](clients.md#redis-client) is set up properly.

## Postgres

This backend uses an external Postgres server to lock the states. It's scalable and can be used also with multiple Terraform backend server instances.

### Config
Set `LOCK_BACKEND` to `postgres`.

Make sure that the [Postgres client](clients.md#postgres-client) is set up properly.
