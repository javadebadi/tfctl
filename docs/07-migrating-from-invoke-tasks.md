---
layout: default
title: Moving from invoke-tasks
render_with_liquid: false
nav_order: 8
---

# 7. Moving from invoke-tasks

tfctl replaces the `infra.*` tasks of
[invoke-tasks](https://github.com/Human-Knowledge-Graph/invoke-tasks)
(`build_infra_collection`). It reads the same `infra.yaml`, so a project moves
over without touching it.

## Steps

1. Install tfctl ([your first project](01-first-project.md#install-tfctl)).
2. Check the config: `tfctl validate-config`. tfctl reads the `variable` blocks
   of every `.tf` file in the folder, where invoke-tasks only read
   `variables.tf`, so it may find a mismatch the old tasks missed.
3. Run `tfctl plan --env prod`. It should show the same plan as
   `uv run invoke infra.plan --env prod`.
4. In `tasks.py`, remove `build_infra_collection` and its import. Keep the rest
   (`ns_code`, your own tasks).
5. If your own Python code calls `load_infra_config`, read `infra.yaml` with
   `yaml.safe_load` instead, and get outputs with
   `tfctl raw-output --env <env> <name>` (see
   [scripts and CI](04-scripts-and-ci.md#use-an-output-in-a-script)).
6. Replace `uv run invoke infra.` in READMEs, Makefiles and comments, using the
   table below.

## The commands

| invoke-tasks | tfctl |
|---|---|
| `invoke infra.create-backend-bucket --env E` | `tfctl create-backend-bucket --env E` |
| `invoke infra.get-backend-bucket-name --env E` | `tfctl backend-bucket-name --env E` |
| `invoke infra.set-cloud-provider --env E` | `tfctl set-cloud-provider --env E` |
| `invoke infra.init --env E` | `tfctl init --env E` |
| `invoke infra.validate --env E` | `tfctl validate --env E` |
| `invoke infra.plan --env E` | `tfctl plan --env E` |
| `invoke infra.apply --env E [--auto-approve]` | `tfctl apply --env E [--auto-approve]` |
| `invoke infra.destroy --env E [--auto-approve]` | `tfctl destroy --env E [--auto-approve]` |
| `invoke infra.refresh --env E` | `tfctl refresh --env E` |
| `invoke infra.output --env E` | `tfctl output --env E` |
| `invoke infra.raw-output --env E --output NAME` | `tfctl raw-output --env E NAME` |
| `invoke infra.show --env E [--json]` | `tfctl show --env E [--json]` |
| `invoke infra.import --env E --address A --resource-id ID` | `tfctl import --env E A ID` |
| `invoke infra.state-list --env E` | `tfctl state list --env E` |
| `invoke infra.state-show --env E --resource A` | `tfctl state show --env E A` |
| `invoke infra.state-mv --env E --source A --destination B` | `tfctl state mv --env E A B` |
| `invoke infra.state-remove --env E --resource A` | `tfctl state rm --env E A` |
| `invoke infra.workspace-list --env E` | `tfctl workspace list --env E` |
| `invoke infra.workspace-new --env E --name N` | `tfctl workspace new --env E N` |
| `invoke infra.workspace-select --env E --name N` | `tfctl workspace select --env E N` |
| `invoke infra.workspace-delete --env E --name N` | `tfctl workspace delete --env E N` |
| `invoke infra.workspace-show --env E` | `tfctl workspace show --env E` |
| `invoke infra.providers --env E` | `tfctl providers --env E` |
| `invoke infra.graph --env E` | `tfctl graph --env E` |
| `invoke infra.console --env E` | `tfctl console --env E` |
| `invoke infra.force-unlock --env E --lock-id L` | `tfctl force-unlock --env E L` |
| `invoke infra.get --env E` | `tfctl get --env E` |
| `invoke infra.fmt` | `tfctl fmt` |
| `invoke infra.validate-yaml-config` | `tfctl validate-config` |

## What behaves differently

- **No `uv` needed.** tfctl is one program for every project, found on your
  `PATH`; there is nothing to add to `pyproject.toml`.
- **Outputs can be captured.** `infra.raw-output` printed init's banner with the
  value; `tfctl raw-output` prints only the value, so the workarounds that called
  `terraform -chdir=... output -raw` directly can go.
- **Exit codes are terraform's**, so `plan -detailed-exitcode` works.
- **tfvars keep their types.** Numbers and bools are written as numbers and
  bools, not strings, and keys are sorted. Terraform accepts both forms, so the
  plan doesn't change.
- **tfvars are checked against every `.tf` file**, not only `variables.tf`.
- **On AWS the profile reaches terraform.** `infra.set-cloud-provider` ran
  `export AWS_PROFILE` in a shell that closed straight away, so it never had an
  effect; tfctl sets it for every terraform command.
- **GCP state buckets are created in `region`** from `backend_buckets`, with
  `gcloud storage`. invoke-tasks used `gsutil mb` without a location (the `US`
  multi-region). Existing buckets are not touched.
