# Migrating to v0.3.0

v0.3.0 changes how states are stored. Upgrading from an older version needs a short downtime, and can't be rolled back
without restoring a backup.

## What changes

Versions before v0.3.0 joined project and state name with `-` and stored the state under the hash of the result. This name
is ambiguous: the states `a-b/c` and `a/b-c` share one object and overwrite each other. With JWT auth, this also let a token
for project `a` access the state `c` of project `a-b`.

Since v0.3.0, states are stored under an unambiguous name, together with metadata (see
[Stored format](storage.md#stored-format)). The server still reads a state from its old name until the state is migrated.

## Upgrade

1. **Back up the storage**, for example:
   - `fs`: copy the directory set in `STORAGE_FS_DIR`.
   - `s3`: copy the bucket, for example with `mc mirror`.
   - `postgres`: `pg_dump --table=states`, or the table set in `STORAGE_POSTGRES_TABLE`.
2. **Make sure no Terraform run is in progress.** Locks taken by the old version are ignored by the new one.
3. **Stop all instances of the old version**, for example with `kubectl scale deployment/terraform-backend --replicas=0`.
   Old and new versions must not run at the same time: old instances don't see states written by new ones and return
   outdated states, and they don't see locks taken by new ones.
4. **Start the new version.**

## Migrating states

A state is migrated the next time a Terraform run locks it, such as `terraform plan` or `terraform apply`, even without
changes. Commands that don't lock the state, such as `terraform state pull` or a run with `-lock=false`, read it from its
old name and don't migrate it.

The server can't migrate states on its own: the old name is a hash, so it doesn't know the project and state name, and
with basic auth the password, that the new name is derived from. To migrate all states, run `terraform plan` once in
every configuration that uses the backend.

## Checking the migration

`check-migration` lists the states that haven't been migrated yet. It needs the same storage configuration as the server,
but no lock backend or KMS:

```sh
terraform-backend check-migration
```

With the container image, give the command the same env and, for the `fs` storage, the same volume as the server:

```sh
docker run --rm --env-file terraform-backend.env -v terraform-backend-states:/states \
  ghcr.io/nimbolus/terraform-backend:0.3.0 check-migration
```

With docker compose, run it in the server's service, which has both:

```sh
docker compose run --rm terraform_backend check-migration
```

If the command finds no states at all, it fails, because that usually means a wrong storage configuration, such as a
missing volume.

The output lists the stored name of each state that hasn't been migrated. The names are hashes, so the command can't show
which project or state they belong to:

```
not migrated: f58e3068eb00033d4b21b3de90a7830ce432e428082fd25fa73ed71808de9b45
1 of 12 states not migrated
```

| Exit code | Meaning                                        |
|-----------|------------------------------------------------|
| `0`       | All states are migrated                        |
| `1`       | Some states aren't migrated yet                |
| `2`       | The check failed, or the storage has no states |

## Things to know

- **No downgrade:** older versions don't find migrated states and return `404`, and Terraform treats a missing state as
  empty. To go back, restore the backup. Changes to states since the upgrade are lost then.
- **States that shared an object:** if two configurations used names like `a-b/c` and `a/b-c`, they already shared one
  state before the upgrade. Usually the first of them that locks it after the upgrade gets this state, and the other one
  starts without a state. If both lock it at the same time, both get a copy. Review the plan of such configurations
  before applying it.
- **Access until migrated:** while a state hasn't been migrated, its old name is still ambiguous. With JWT auth, a token
  for project `a` can read, migrate and delete the state `a-b/c` through `a/b-c`. Migrate all states to close this.
- **Removal in v1.0.0:** v1.0.0 won't read states from their old name anymore. Run `check-migration` before upgrading to
  it, since states that aren't migrated by then appear empty.
