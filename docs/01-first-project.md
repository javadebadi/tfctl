---
layout: default
title: Your first project
render_with_liquid: false
nav_order: 2
---

# 1. Your first project

You will install tfctl, describe one environment of a project on Google Cloud,
and create its infrastructure.

## What you need

- Go, to build tfctl.
- [Terraform](https://developer.hashicorp.com/terraform/install).
- The cloud's command line tool, logged in: `gcloud` for Google Cloud (run
  `gcloud auth login` and `gcloud auth application-default login`), or `aws` for
  AWS (see [GCP and AWS](05-gcp-and-aws.md)).

## Install tfctl

Once per computer, not per project:

```bash
git clone git@github.com:javadebadi/tfcli.git
cd tfcli
make local-build-install
```

This builds `tfctl`, copies it to `/usr/local/bin` (it asks for your password)
and adds zsh completion to `~/.zshrc`, so `tfctl plan --env <TAB>` offers the
environments of the project you are in. Run it again after you pull changes.

Check it:

```bash
tfctl --help
```

## The project

A project that tfctl manages looks like this:

```
my-project/
  infra.yaml          # your environments; tfctl reads this
  terraform/
    backend.tf        # says *which kind* of state storage, not which bucket
    variables.tf      # the variables your code needs
    main.tf           # your resources
    prod.tfvars       # written by tfctl; never edit it, add it to .gitignore
```

`backend.tf` names only the kind of backend. tfctl fills in the bucket when it
runs `terraform init`, so the same code works for every environment:

```hcl
terraform {
  backend "gcs" {}
}
```

Your `variables.tf` declares what the code needs:

```hcl
variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}
```

## The infra.yaml

Save this as `infra.yaml`, next to the `terraform/` folder:

```yaml
envs:
  prod:
    hosted_on: GCP
    gcp_project_id: my-project-123
    infra_dir: terraform              # relative to this file

backend_buckets:
  prod:
    hosted_on: GCP
    bucket_name: my-project-123-terraform-state-prod
    region: us-central1

tfvars:
  prod:
    project_id: my-project-123
    region: us-central1
```

There are three sections, and each has one entry per environment:

- `envs`: where the environment lives and where its Terraform code is.
- `backend_buckets`: the bucket that keeps the environment's Terraform state.
  Bucket names are global on each cloud, so put the project in the name.
- `tfvars`: the values of your Terraform variables for that environment.

Check it:

```bash
tfctl validate-config
```

```
infra.yaml is valid.
```

`validate-config` also compares `tfvars` with your `variable` blocks. Had you
left out `project_id`, which has no default, it would have said so.

## Create the state bucket

Once per environment, before anything else:

```bash
tfctl create-backend-bucket --env prod
```

If you are not sure whether it exists already, use
`tfctl create-backend-bucket-if-not-exists --env prod`, which does nothing when it does.

## Plan and apply

```bash
tfctl plan --env prod
```

You will see three things go by: `wrote .../terraform/prod.tfvars`, the output
of `terraform init`, and then the plan. When the plan is what you expect:

```bash
tfctl apply --env prod
```

Terraform shows the plan again and asks you to type `yes`. In a script, add
`--auto-approve` to skip the question.

## What you have now

- Your state is in the bucket, not on your laptop, so anyone with access to the
  project can run the same commands and see the same state.
- `prod.tfvars` is written again on every run, so `infra.yaml` is the only
  place where the values live. Commit `infra.yaml`; ignore `*.tfvars`.
- You never typed a `-backend-config` or `-var-file` flag.

Next: [the infra.yaml file](02-infra-yaml.md), in full.
