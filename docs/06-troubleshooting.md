---
layout: default
title: Troubleshooting
render_with_liquid: false
nav_order: 7
---

# 6. Troubleshooting

Each entry shows what came back, what it means, and the fix. Messages that
start with `Error:` come from tfctl, before terraform runs; anything else comes
from terraform or the cloud's tool, which tfctl passes on unchanged.

## "could not find infra.yaml"

```
Error: could not find infra.yaml in the current directory or any parent; pass --config
```

You are outside the project. tfctl looks in the current folder and its parents,
never in sibling or child folders. `cd` into the project, or pass
`--config path/to/infra.yaml`.

## "--env is required"

```
Error: --env is required (available: dev, prod)
```

Every command that runs terraform needs an environment. The message lists the
ones in your `infra.yaml`. (`fmt` and `validate-config` don't need one.)

## "unknown environment"

```
Error: unknown environment "staging" (available: dev, prod)
```

There is no `staging` under `envs`. Check the spelling, or add the environment:
it needs an entry in `envs`, `backend_buckets` and `tfvars`.

## The infra.yaml is invalid

```
Error: error loading config: invalid /home/me/my-project/infra.yaml:
env "prod" is missing infra_dir
env "prod" has invalid hosted_on "AZURE" (must be AWS or GCP)
env "prod" has no matching entry in backend_buckets
```

tfctl lists every problem at once. Fix them all and run
`tfctl validate-config` to check.

## "tfvars do not match terraform variables"

```
Error: tfvars do not match terraform variables:
env "prod": missing required variables: project_id
env "prod": keys not declared as variables in /home/me/my-project/terraform: regoin
```

- **missing required variables**: a `variable` block in your code has no
  `default`, and `tfvars.prod` in `infra.yaml` doesn't set it. Add it to
  `infra.yaml`, or give the variable a default.
- **keys not declared as variables**: `infra.yaml` sets something your code
  doesn't declare, usually a typo (`regoin`) or a variable you removed from the
  code. Fix or remove the key.

Only `.tf` files directly in `infra_dir` are read, as Terraform does; variables
of a module in a subfolder are that module's own business.

## "bucket doesn't exist"

```
Error: Failed to get existing workspaces: querying Cloud Storage failed: storage: bucket doesn't exist
```

On AWS the message mentions S3 and `NoSuchBucket` or "does not exist". The
environment's state bucket hasn't been created. Run, once:

```bash
tfctl create-backend-bucket --env prod
```

If you believe it exists, compare `tfctl backend-bucket-name --env prod` with
the cloud console, and check you are logged in to the right account.

## "Error acquiring the state lock"

Terraform locks the state while it works. If someone else is running, wait. If
an earlier run crashed or was killed, the lock stays; the message shows its
`ID`. When you are sure nothing is running:

```bash
tfctl force-unlock --env prod <ID>
```

## Credentials errors

- GCP, `could not find default credentials`: run
  `gcloud auth application-default login`. Logging in with `gcloud auth login`
  alone is not enough for Terraform.
- AWS, `failed to get shared config profile`: the `aws_profile` of the
  environment is not in `~/.aws/config`. Check with
  `tfctl set-cloud-provider --env prod`.

## The lock file changed

tfctl runs `terraform init -upgrade`, so when a provider's version constraint
allows a newer release, `.terraform.lock.hcl` is updated to it. Review and
commit that change as you would any other; pin the provider version in your code
if you want it to stay put.

## A plan shows changes nobody made

The state belongs to one environment and the `.tfvars` is written fresh on
every run, so the usual cause is that someone changed the resource in the cloud
console. `tfctl plan --env prod -- -refresh-only` shows only what drifted.
