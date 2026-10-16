---
layout: default
title: Everyday commands
render_with_liquid: false
nav_order: 4
---

# 3. Everyday commands

Every command below takes `--env`, writes `<env>.tfvars` first and runs
`terraform init` before the terraform command itself. `tfctl <command> --help`
shows the details of each.

## Plan, apply, destroy

```bash
tfctl plan --env prod
tfctl apply --env prod                  # asks you to type yes
tfctl apply --env prod --auto-approve   # doesn't ask
tfctl destroy --env prod
tfctl refresh --env prod                # update the state from what really exists
```

## Pass more flags to terraform

Anything after `--` goes to terraform as it is:

```bash
tfctl plan --env prod -- -target=google_sql_database_instance.main
tfctl apply --env prod -- -replace=google_cloud_run_v2_service.api
tfctl plan --env prod -- -refresh-only
```

## Outputs

```bash
tfctl output --env prod                 # all of them
tfctl output --env prod --json          # all of them, as JSON
tfctl raw-output --env prod backend_url # one value, nothing else
```

`raw-output` prints only the value, so it can be used in a shell; see
[scripts and CI](04-scripts-and-ci.md).

## Look at and change the state

```bash
tfctl state list --env prod
tfctl state show --env prod google_cloud_run_v2_service.api
tfctl show --env prod                   # the whole state; --json for JSON
```

When you rename a resource in your code, move it in the state, or Terraform
will destroy the old one and create a new one:

```bash
tfctl state mv --env prod google_storage_bucket.files google_storage_bucket.uploads
```

To stop managing something without destroying it:

```bash
tfctl state rm --env prod google_storage_bucket.legacy
```

## Bring an existing resource under Terraform

Write the resource block first, then:

```bash
tfctl import --env prod google_storage_bucket.uploads my-project-123-uploads
```

The second argument is the resource's ID in the cloud; the documentation of each
resource type says what form it takes.

## Workspaces

```bash
tfctl workspace list --env prod
tfctl workspace new --env prod experiment
tfctl workspace select --env prod experiment
tfctl workspace show --env prod
tfctl workspace delete --env prod experiment
```

With tfctl you usually don't need workspaces: each environment already has its
own state bucket.

## Everything else

| command | does |
|---|---|
| `tfctl validate --env prod` | checks the code is valid |
| `tfctl providers --env prod` | the providers and their version constraints |
| `tfctl graph --env prod \| dot -Tsvg > graph.svg` | a picture of how resources depend on each other |
| `tfctl console --env prod` | try out expressions, with prod's variables |
| `tfctl force-unlock --env prod LOCK_ID` | release a lock left by a run that crashed |
| `tfctl get --env prod` | update modules (doesn't run init) |
| `tfctl fmt` | format the code of every environment (`--env` for one) |
| `tfctl set-cloud-provider --env prod` | point `gcloud` at prod's project (on AWS: check the profile) |
| `tfctl tfvars --env prod` | only write `prod.tfvars` |

For a terraform command that has no tfctl command, use `tf`. It runs in the
environment's folder, after init, with `AWS_PROFILE` set on AWS:

```bash
tfctl tf --env prod -- state pull > backup.tfstate
```

`tf` adds no `-var-file`, so pass `-var-file=prod.tfvars` yourself when the
command needs variables.

## Don't run terraform directly

Running `terraform plan` yourself in the folder skips everything tfctl does: the
`.tfvars` may be stale, and `init` may point at another environment's state from
an earlier run. If tfctl lacks something, use `tfctl tf`.

Next: [scripts and CI](04-scripts-and-ci.md).
