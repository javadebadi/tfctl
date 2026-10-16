---
layout: default
title: GCP and AWS
render_with_liquid: false
nav_order: 6
---

# 5. GCP and AWS

The commands are the same on both clouds. What differs is how tfctl creates
the state bucket, what it gives `terraform init`, and how it picks your
credentials.

| | GCP | AWS |
|---|---|---|
| `backend.tf` | `terraform { backend "gcs" {} }` | `terraform { backend "s3" {} }` |
| state location | `gs://<bucket>/terraform/state/` | `s3://<bucket>/terraform/state` |
| `init` gets | `bucket`, `prefix=terraform/state` | `bucket`, `key=terraform/state`, `region` |
| credentials | your `gcloud` application default login | the env's `aws_profile`, given to terraform as `AWS_PROFILE` |
| `create-backend-bucket` runs | `gcloud storage buckets create gs://<bucket> --project <gcp_project_id> --location <region>` | `aws s3api create-bucket --bucket <bucket> --profile <aws_profile>` (with the region) |
| `set-cloud-provider` | `gcloud config set project <gcp_project_id>` | `aws sts get-caller-identity --profile <aws_profile>` |

## GCP

```yaml
envs:
  prod:
    hosted_on: GCP
    gcp_project_id: my-project-123
    infra_dir: terraform
backend_buckets:
  prod:
    hosted_on: GCP
    bucket_name: my-project-123-terraform-state-prod
    region: us-central1
```

Terraform's Google provider and the `gcs` backend use your *application default*
credentials, which are not the same as the `gcloud` login. Log in to both:

```bash
gcloud auth login
gcloud auth application-default login
```

`gcp_project_id` is where the state bucket is created. Your Terraform code still
needs its own project setting (usually a `project_id` variable in `tfvars`).

## AWS

```yaml
envs:
  prod:
    hosted_on: AWS
    aws_profile: prod
    infra_dir: infra
backend_buckets:
  prod:
    hosted_on: AWS
    bucket_name: my-site-terraform-state-prod
    region: ca-central-1
```

`aws_profile` is a profile in `~/.aws/config`. tfctl sets `AWS_PROFILE` for the
terraform commands it runs, so you don't export anything, and two terminals can
work on two accounts at once. `set-cloud-provider` cannot change your own
shell's `AWS_PROFILE` (no program can change its parent's environment); it
checks the profile works and prints the `export` line for you to run, should
you want to use `aws` yourself.

`region` matters on AWS: it is where the bucket is created, and `init` needs it
to find the bucket.

## A project on both

An environment's `hosted_on` and its bucket's `hosted_on` are separate. A
project on Google Cloud that also manages DNS in AWS Route 53 is still a GCP
environment: its state is in a GCS bucket, and the `aws` provider in the code
finds its credentials in its usual ways (a `profile` in the provider block, or
the environment).

Next: [troubleshooting](06-troubleshooting.md).
