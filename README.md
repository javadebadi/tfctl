# tfctl

**New here? Read the [tutorial](https://javadebadi.github.io/tfctl/).** This page is the reference.

tfctl is a thin wrapper around Terraform that makes managing environments (prod, staging, dev, ...) easier. You describe each environment once in an `infra.yaml` file (its cloud, its state bucket, its variables) and then run `tfctl plan --env prod` instead of remembering the backend and var-file flags for each.

Before every command that takes `--env`, tfctl:

1. checks the env's `tfvars` in `infra.yaml` against the `variable` blocks in the infra dir's `*.tf` files (each variable without a default must be set, each key must be declared),
2. regenerates `<infra_dir>/<env>.tfvars` from them (never edit that file by hand; gitignore it),
3. runs `terraform init -upgrade -reconfigure` with the env's backend bucket, for commands that talk to terraform state.

## Install

Once per computer; every project then uses the same `tfctl`:

```bash
git clone git@github.com:javadebadi/tfctl.git && cd tfctl
make local-build-install   # builds, copies to /usr/local/bin and adds zsh completion
```

## infra.yaml

tfctl looks for `infra.yaml` in the current directory and then each parent; `--config` overrides that. `infra_dir` is relative to the directory of `infra.yaml`.

```yaml
envs:
  prod:
    hosted_on: GCP            # GCP or AWS
    gcp_project_id: my-project
    aws_profile: null         # for AWS: passed to terraform as AWS_PROFILE
    infra_dir: terraform

backend_buckets:              # where the terraform state lives, one per env
  prod:
    hosted_on: GCP
    bucket_name: my-project-terraform-state-prod
    region: us-central1

tfvars:                       # one per env; strings, numbers, bools, lists and maps
  prod:
    project_id: my-project
    region: us-central1
    admin_emails:
      - someone@example.com
```

The state is stored under the prefix (GCS) or key (S3) `terraform/state`, so the terraform code needs an empty backend block such as `terraform { backend "gcs" {} }`.

## Commands

| Command | What it does |
|---|---|
| `create-backend-bucket --env E` | Create the state bucket (once, before the first init) |
| `create-backend-bucket-if-not-exists --env E` | Same, but does nothing if it exists |
| `backend-bucket-name --env E` | Print the state bucket name |
| `set-cloud-provider --env E` | `gcloud config set project`, or check the AWS profile works |
| `tfvars --env E` | Only regenerate `<env>.tfvars` |
| `validate-config` | Check `infra.yaml`, including every env's tfvars against its variables |
| `init --env E` | `terraform init` with the backend config |
| `plan`, `apply`, `destroy`, `refresh --env E` | With `-var-file=<env>.tfvars`; `apply`/`destroy` take `--auto-approve` |
| `validate`, `providers`, `graph`, `console --env E` | The terraform commands of the same name |
| `output --env E [NAME]` | `--raw`, `--json` |
| `raw-output --env E NAME` | Same as `output --raw NAME` |
| `show --env E` | `--json` |
| `import --env E ADDRESS ID` | |
| `state list\|show\|mv\|rm --env E ...` | |
| `workspace list\|show\|new\|select\|delete --env E ...` | |
| `force-unlock --env E LOCK_ID` | |
| `get --env E` | Update modules (no init) |
| `fmt [--check] [--env E]` | Format the infra dir of every env (or just E); `check-autoformat` is `fmt --check` |
| `tf --env E -- ARGS...` | Any other terraform command, in the env's infra dir, after init |

Arguments after `--` are passed to terraform, e.g. `tfctl plan --env prod -- -target=module.db`.

Terraform's exit code is passed through (so `tfctl plan --env prod -- -detailed-exitcode` works in CI), and the output of `init` goes to stderr, so a value can be captured:

```bash
URL=$(tfctl raw-output --env prod backend_url)
```

## Layout

```
docs/                 the tutorial, published at https://javadebadi.github.io/tfctl/
cmd.go                the commands and flags
config.go             infra.yaml: finding, parsing and validating it
tfvars.go             writing <env>.tfvars and checking it against the variable blocks
tf.go                 terraform init, fmt and get
backend_bucket.go     creating the state bucket, set-cloud-provider
helpers.go            running terraform and the cloud CLIs
```

## Development

```bash
make test    # or: make check (fmt, vet, lint, coverage)
```

The tests need no cloud login and no terraform. To try a change, `go build -o
/tmp/tfctl .` and run it in a project; nothing else on your machine changes.
