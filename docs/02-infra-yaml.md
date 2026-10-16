---
layout: default
title: The infra.yaml file
render_with_liquid: false
nav_order: 3
---

# 2. The infra.yaml file

## How tfctl finds it

tfctl looks for `infra.yaml` in the folder you run it from, then in each parent
folder, and uses the first one it finds. So in a project laid out as

```
my-project/
  infra.yaml
  terraform/
    modules/
```

`tfctl plan --env prod` works from `my-project/`, `terraform/` or `modules/`.
From anywhere else, name the file:

```bash
tfctl --config ~/code/my-project/infra.yaml plan --env prod
```

Folders in the file (`infra_dir`) are relative to the folder that holds
`infra.yaml`, not to where you are.

## A complete example

Two environments, one on each cloud:

```yaml
envs:
  prod:
    hosted_on: GCP
    gcp_project_id: my-project-123
    infra_dir: terraform
  staging:
    hosted_on: AWS
    aws_profile: staging
    infra_dir: terraform

backend_buckets:
  prod:
    hosted_on: GCP
    bucket_name: my-project-123-terraform-state-prod
    region: us-central1
  staging:
    hosted_on: AWS
    bucket_name: my-project-terraform-state-staging
    region: ca-central-1

tfvars:
  prod:
    project_id: my-project-123
    region: us-central1
  staging:
    region: ca-central-1
```

## envs

One entry per environment. The name (`prod`, `staging`) is what you pass to
`--env`; upper or lower case both work.

| field | required | meaning |
|---|---|---|
| `hosted_on` | yes | `GCP` or `AWS` (any case) |
| `infra_dir` | yes | the folder with this environment's Terraform code, relative to `infra.yaml` |
| `gcp_project_id` | on GCP | used to create the state bucket and by `set-cloud-provider` |
| `aws_profile` | on AWS | the profile in `~/.aws/config`; tfctl gives it to terraform as `AWS_PROFILE` |

Several environments may share one `infra_dir`. Each gets its own
`<env>.tfvars` there, and its own state, because the state bucket differs.

## backend_buckets

One entry per environment, every environment needs one.

| field | required | meaning |
|---|---|---|
| `hosted_on` | yes | `GCP` (a Cloud Storage bucket) or `AWS` (an S3 bucket) |
| `bucket_name` | yes | the bucket |
| `region` | no | where `create-backend-bucket` puts it; on AWS also given to `terraform init` |

Inside the bucket the state is under `terraform/state`.

## tfvars

One entry per environment, every environment needs one (an empty `{}` is fine
when your code has no variables). Each key must be a variable your code
declares, and every variable without a `default` must be set. tfctl reads the
`variable` blocks of every `.tf` file in `infra_dir` (not in subfolders, which
is how Terraform itself works) and refuses to run when they disagree:

```
Error: tfvars do not match terraform variables:
env "prod": missing required variables: project_id
env "prod": keys not declared as variables in /home/me/my-project/terraform: regoin
```

YAML values keep their type in the written `.tfvars`:

```yaml
tfvars:
  prod:
    name: my-app               # a string
    replicas: 3                # a number
    public: true               # a bool
    admins:                    # a list
      - me@example.com
    labels:                    # a map
      team: core
    certificate: null          # null: use the variable's default
```

becomes

```hcl
admins = [
  "me@example.com",
]
certificate = null
labels = {
  "team" = "core"
}
name     = "my-app"
public   = true
replicas = 3
```

Keys are sorted, and the file is formatted like `terraform fmt` would, so
`tfctl fmt --check` never complains about it. A `${` in a string is written as
`$${`, so a value is always taken literally, never as a Terraform expression.

## What does not belong here

**Secrets.** `infra.yaml` is committed and `.tfvars` sits on disk. Let Terraform
create a secret's container (for example a Secret Manager secret) and set its
value with the cloud's own tool, or let Terraform generate the value itself
(such as `random_password`).

Next: [everyday commands](03-everyday-commands.md).
