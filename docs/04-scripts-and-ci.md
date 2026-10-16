---
layout: default
title: Scripts and CI
render_with_liquid: false
nav_order: 5
---

# 4. Scripts and CI

## Use an output in a script

tfctl writes everything that is not the answer (the `wrote prod.tfvars` line,
the output of `terraform init`) to stderr. Only the value goes to stdout:

```bash
BACKEND_URL=$(tfctl raw-output --env prod backend_url)
gcloud builds submit . --substitutions=_API_BASE_URL="$BACKEND_URL"
```

The same from a Makefile:

```make
invalidate:
	$(eval DISTRIBUTION_ID := $(shell tfctl raw-output --env prod cloudfront_distribution_id))
	aws cloudfront create-invalidation --distribution-id "$(DISTRIBUTION_ID)" --paths "/*"
```

and from Python:

```python
import subprocess

url = subprocess.run(
    ["tfctl", "raw-output", "--env", "prod", "backend_url"],
    capture_output=True, text=True, check=True,
).stdout.strip()
```

If the script may run outside the project's folders, add
`--config /path/to/infra.yaml`.

## Exit codes

tfctl exits with terraform's own exit code, so a script can tell what happened:

- `0`: it worked.
- `1`: it failed (a terraform error, or a problem tfctl found first, such as a
  tfvars mismatch; then the message starts with `Error:`).
- `2` from `plan -detailed-exitcode`: it worked and there are changes.

```bash
tfctl plan --env prod -- -detailed-exitcode
case $? in
  0) echo "no changes" ;;
  2) echo "changes to apply" ;;
  *) echo "plan failed"; exit 1 ;;
esac
```

## Checks for a pipeline

Two checks need no cloud login at all:

```bash
tfctl validate-config   # infra.yaml is well formed, tfvars match the variables
tfctl fmt --check       # the terraform files are formatted (shows the diff if not)
```

A GitHub Actions job:

```yaml
name: Terraform checks

on: [pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - uses: hashicorp/setup-terraform@v3
      - name: Install tfctl
        run: |
          git clone --depth 1 https://github.com/javadebadi/tfcli "$RUNNER_TEMP/tfcli"
          (cd "$RUNNER_TEMP/tfcli" && go build -o "$HOME/bin/tfctl" .)
          echo "$HOME/bin" >> "$GITHUB_PATH"
      - run: tfctl validate-config
      - run: tfctl fmt --check
```

A plan in CI also needs credentials for the state bucket and the cloud, for
example through `google-github-actions/auth` or `aws-actions/configure-aws-credentials`;
after that step, `tfctl plan --env prod` works as on your laptop. On AWS, tfctl
sets `AWS_PROFILE` from `infra.yaml`, so the CI credentials must be available
under that profile name, or leave `aws_profile` out for that environment.

Next: [GCP and AWS](05-gcp-and-aws.md).
