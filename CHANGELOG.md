# Changelog

All notable changes to this project are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-10-16

The first version that replaces the `infra.*` tasks of invoke-tasks.

### Added

- Every command of the invoke tasks: `apply`, `destroy`, `refresh`, `output`,
  `raw-output`, `show`, `import`, `state list|show|mv|rm`,
  `workspace list|show|new|select|delete`, `providers`, `graph`, `console`,
  `force-unlock`, `get`, `fmt`, `validate-config`, `backend-bucket-name` and
  `set-cloud-provider`.
- `tf --env E -- ARGS`, for any terraform command without a tfctl command of its
  own, and arguments after `--` passed to terraform by `plan`, `apply` and the
  others.
- AWS: creating the state bucket in S3, the S3 backend settings for `init`, and
  the env's `aws_profile` given to terraform as `AWS_PROFILE`.
- The tfvars in `infra.yaml` are checked against the `variable` blocks of every
  `.tf` file in the infra dir before a command runs.
- `infra.yaml` is found in the current folder or a parent, so `--config` is
  optional; `infra_dir` is relative to the file.
- Completion of `--env` with the project's environments.
- A tutorial in `docs/`, published with GitHub Pages.

### Changed

- tfvars keep their YAML types (lists, maps, numbers, bools, null), are sorted,
  and are formatted like `terraform fmt`. Before, every value was written as a
  quoted string, which broke lists and maps.
- The output of `terraform init` goes to stderr, so the value printed by
  `raw-output` can be captured.
- tfctl exits with terraform's exit code; before, some failures exited with 0.
- GCP state buckets are created with `gcloud storage`, in the configured region.
- `fmt` and `check-autoformat` no longer need `--env` or run `init`.
- `--env` and `hosted_on` are matched case-insensitively.

### Fixed

- `apply` and `destroy` can ask for approval: terraform now gets the terminal's
  input.
- `create-backend-bucket-if-not-exists` creates the bucket when it is missing,
  instead of failing.
