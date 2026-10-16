---
layout: default
title: Tutorial
render_with_liquid: false
nav_order: 1
permalink: /
---

# tfctl tutorial

tfctl is a thin wrapper around Terraform that makes managing environments
(prod, staging, dev, ...) easier. You describe your environments once, in a file called `infra.yaml`: which cloud each one is on,
where its Terraform state is kept, and the values of its Terraform variables.
After that, `tfctl plan --env prod` does everything that a bare `terraform plan`
would leave to you:

- it writes `prod.tfvars` from `infra.yaml`, after checking that every variable
  your Terraform code needs is set and no unknown one is,
- it runs `terraform init` against the right state bucket,
- it runs `terraform plan` with `-var-file=prod.tfvars`, in the right folder.

One install serves every project on your machine. tfctl finds the project's
`infra.yaml` by looking in the folder you are in, then in each parent folder.

Read the pages in order the first time; each one builds on the previous one.

| | Page | You learn |
|---|---|---|
| 1 | [Your first project](01-first-project.md) | install tfctl, write an `infra.yaml`, create the state bucket, plan and apply |
| 2 | [The infra.yaml file](02-infra-yaml.md) | every field, how values become tfvars, how the file is found |
| 3 | [Everyday commands](03-everyday-commands.md) | outputs, state, imports, workspaces, passing extra flags to terraform |
| 4 | [Scripts and CI](04-scripts-and-ci.md) | capturing an output, exit codes, checks for a pipeline |
| 5 | [GCP and AWS](05-gcp-and-aws.md) | what tfctl does differently on each cloud |
| 6 | [Troubleshooting](06-troubleshooting.md) | the errors you will meet, what they mean, the fix |

The [README](https://github.com/javadebadi/tfcli#readme) is the reference; these
pages are the walkthrough.
