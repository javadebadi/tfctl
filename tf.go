package main

import (
	"fmt"
	"os"
)

func tfInitArgs(c Config, env string) ([]string, error) {
	bucket, ok := c.BackendBuckets[env]
	if !ok {
		return nil, fmt.Errorf("no bucket info for env %v", env)
	}

	args := []string{
		"-input=false",
		"-upgrade",
		"-reconfigure",
		fmt.Sprintf("-backend-config=bucket=%s", bucket.BucketName),
	}

	switch bucket.HostedOn {
	case GCP:
		args = append(args, "-backend-config=prefix=terraform/state")
	case AWS:
		args = append(args, "-backend-config=key=terraform/state")
		if bucket.Region != "" {
			args = append(args, fmt.Sprintf("-backend-config=region=%s", bucket.Region))
		}
	}
	return args, nil
}

// tfInit runs terraform init with the backend config of env. Its output goes
// to stderr so the stdout of the command that follows (e.g. output -raw)
// stays clean enough to capture.
func tfInit(c Config, env string) error {
	args, err := tfInitArgs(c, env)
	if err != nil {
		return err
	}
	return runTerraformTo(c, env, os.Stderr, "init", args...)
}

func tfFmt(dir string, check bool) error {
	args := []string{"fmt", "-recursive"}
	if check {
		// to check autoformat is run (can be used in CI pipelines)
		args = append(args, "-check", "-diff")
	}
	return command{name: "terraform", args: args, dir: dir}.run()
}

// tfGet downloads and updates modules without touching the backend
func tfGet(c Config, env string) error {
	return runTerraform(c, env, "get", "-update")
}
