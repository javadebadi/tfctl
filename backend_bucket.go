// tools to manage terraform backend bucket used to store state of terraform
package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func createGcpBucket(bucketName string, gcpProjectId string, region string) error {
	// Runs this gcloud command
	// gcloud storage buckets create gs://bucketName --project=gcpProjectId [--location=region]
	args := []string{"storage", "buckets", "create", fmt.Sprintf("gs://%s", bucketName), "--project", gcpProjectId}
	if region != "" {
		args = append(args, "--location", region)
	}
	return runCommand("gcloud", args...)
}

func doesGcpBucketExist(bucketName string, gcpProjectId string) (bool, error) {
	out, err := runCommandCaptured("gcloud", "storage", "buckets", "describe", fmt.Sprintf("gs://%s", bucketName), "--project", gcpProjectId, "--format=value(name)")
	return bucketExistence(out, err)
}

func createAwsBucket(bucketName string, awsProfile string, region string) error {
	// Runs this aws command
	// aws s3api create-bucket --bucket bucketName --profile awsProfile [--region region --create-bucket-configuration LocationConstraint=region]
	args := []string{"s3api", "create-bucket", "--bucket", bucketName, "--profile", awsProfile}
	// us-east-1 is the default location and rejects an explicit LocationConstraint
	if region != "" && region != "us-east-1" {
		args = append(args, "--region", region, "--create-bucket-configuration", fmt.Sprintf("LocationConstraint=%s", region))
	}
	return runCommand("aws", args...)
}

func doesAwsBucketExist(bucketName string, awsProfile string) (bool, error) {
	out, err := runCommandCaptured("aws", "s3api", "head-bucket", "--bucket", bucketName, "--profile", awsProfile)
	return bucketExistence(out, err)
}

// bucketExistence interprets the result of a "describe bucket" command: a
// not-found failure means the bucket does not exist, any other failure
// (no credentials, no permission, missing CLI) is an error
func bucketExistence(output string, err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "404") || strings.Contains(lower, "not found") || strings.Contains(lower, "notfound") {
			return false, nil
		}
	}
	return false, fmt.Errorf("checking bucket: %w\n%s", err, strings.TrimSpace(output))
}

func createBackendBucket(c Config, env string) error {
	envConfig := c.Envs[env]
	bucket := c.BackendBuckets[env]

	switch bucket.HostedOn {
	case GCP:
		if envConfig.GCPProjectID == "" {
			return fmt.Errorf("cannot create GCP bucket: no gcp_project_id configured for env %q", env)
		}
		return createGcpBucket(bucket.BucketName, envConfig.GCPProjectID, bucket.Region)
	case AWS:
		if envConfig.AWSProfile == "" {
			return fmt.Errorf("cannot create AWS bucket: no aws_profile configured for env %q", env)
		}
		return createAwsBucket(bucket.BucketName, envConfig.AWSProfile, bucket.Region)
	default:
		return fmt.Errorf("hosted_on %q not implemented", bucket.HostedOn)
	}
}

func doesBackendBucketExist(c Config, env string) (bool, error) {
	envConfig := c.Envs[env]
	bucket := c.BackendBuckets[env]

	switch bucket.HostedOn {
	case GCP:
		if envConfig.GCPProjectID == "" {
			return false, fmt.Errorf("cannot check GCP bucket: no gcp_project_id configured for env %q", env)
		}
		return doesGcpBucketExist(bucket.BucketName, envConfig.GCPProjectID)
	case AWS:
		if envConfig.AWSProfile == "" {
			return false, fmt.Errorf("cannot check AWS bucket: no aws_profile configured for env %q", env)
		}
		return doesAwsBucketExist(bucket.BucketName, envConfig.AWSProfile)
	default:
		return false, fmt.Errorf("hosted_on %q not implemented", bucket.HostedOn)
	}
}

func createBackendBucketIfNotExists(c Config, env string) error {
	exists, err := doesBackendBucketExist(c, env)
	if err != nil {
		return err
	}

	bucketName := c.BackendBuckets[env].BucketName
	if exists {
		fmt.Printf("bucket %v already exists\n", bucketName)
		return nil
	}

	fmt.Printf("bucket %v does not exist, creating it\n", bucketName)
	return createBackendBucket(c, env)
}

// configureCloudProvider points the provider CLI at the env: sets the active
// gcloud project, or checks the AWS profile's credentials
func configureCloudProvider(c Config, env string) error {
	envConfig := c.Envs[env]

	switch envConfig.HostedOn {
	case GCP:
		if envConfig.GCPProjectID == "" {
			return fmt.Errorf("cannot configure GCP: no gcp_project_id configured for env %q", env)
		}
		return runCommand("gcloud", "config", "set", "project", envConfig.GCPProjectID)
	case AWS:
		if envConfig.AWSProfile == "" {
			return fmt.Errorf("cannot configure AWS: no aws_profile configured for env %q", env)
		}
		// a child process cannot change the parent shell's AWS_PROFILE; terraform
		// commands run through this CLI get it automatically, so just verify it
		if err := runCommand("aws", "sts", "get-caller-identity", "--profile", envConfig.AWSProfile); err != nil {
			return err
		}
		fmt.Printf("AWS profile %q works. tfctl passes it to terraform; to use it in your shell run: export AWS_PROFILE=%s\n", envConfig.AWSProfile, envConfig.AWSProfile)
		return nil
	default:
		return fmt.Errorf("hosted_on %q not implemented", envConfig.HostedOn)
	}
}
