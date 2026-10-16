package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTfInitArgs(t *testing.T) {
	config := Config{
		BackendBuckets: map[string]BackendBucket{
			"prod": {HostedOn: GCP, BucketName: "gcp-state", Region: "us-central1"},
			"dev":  {HostedOn: AWS, BucketName: "aws-state", Region: "ca-central-1"},
		},
	}
	base := []string{"-input=false", "-upgrade", "-reconfigure"}

	tests := []struct {
		env  string
		want []string
	}{
		{"prod", append(base, "-backend-config=bucket=gcp-state", "-backend-config=prefix=terraform/state")},
		{"dev", append(base, "-backend-config=bucket=aws-state", "-backend-config=key=terraform/state", "-backend-config=region=ca-central-1")},
	}
	for _, tt := range tests {
		got, err := tfInitArgs(config, tt.env)
		if err != nil {
			t.Fatalf("tfInitArgs(%q) error = %v", tt.env, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("tfInitArgs(%q) = %v, want %v", tt.env, got, tt.want)
		}
	}

	if _, err := tfInitArgs(config, "missing"); err == nil {
		t.Error("expected an error for an env without a backend bucket")
	}
}

func TestTerraformEnv(t *testing.T) {
	config := Config{
		Envs: map[string]Env{
			"aws":         {HostedOn: AWS, AWSProfile: "prof"},
			"aws-noprof":  {HostedOn: AWS},
			"gcp-profile": {HostedOn: GCP, AWSProfile: "prof"},
		},
	}
	if got := terraformEnv(config, "aws"); !reflect.DeepEqual(got, []string{"AWS_PROFILE=prof"}) {
		t.Errorf("terraformEnv(aws) = %v", got)
	}
	if got := terraformEnv(config, "aws-noprof"); got != nil {
		t.Errorf("terraformEnv(aws-noprof) = %v, want nil", got)
	}
	if got := terraformEnv(config, "gcp-profile"); got != nil {
		t.Errorf("terraformEnv(gcp-profile) = %v, want nil", got)
	}
}

func TestBucketExistence(t *testing.T) {
	// a real *exec.ExitError, from a command that fails
	exitErr := exec.Command("sh", "-c", "exit 1").Run()

	tests := []struct {
		name       string
		output     string
		err        error
		wantExists bool
		wantErr    bool
	}{
		{"success", "gs://bucket", nil, true, false},
		{"gcloud not found", "ERROR: (gcloud.storage.buckets.describe) gs://b not found: 404.", exitErr, false, false},
		{"aws not found", "An error occurred (404) when calling the HeadBucket operation: Not Found", exitErr, false, false},
		{"forbidden", "An error occurred (403) when calling the HeadBucket operation: Forbidden", exitErr, false, true},
		{"cli missing", "", errors.New("exec: \"gcloud\": executable file not found in $PATH"), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := bucketExistence(tt.output, tt.err)
			if exists != tt.wantExists || (err != nil) != tt.wantErr {
				t.Errorf("bucketExistence() = %v, %v; want %v, error=%v", exists, err, tt.wantExists, tt.wantErr)
			}
		})
	}
}

func TestFindConfig(t *testing.T) {
	// Given infra.yaml in a parent of the working directory
	root := t.TempDir()
	nested := filepath.Join(root, "terraform", "modules")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "infra.yaml"), []byte("envs: {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// When searching from the nested directory
	got, err := findConfig(nested)

	// Then the parent's infra.yaml is found
	if err != nil {
		t.Fatalf("findConfig() error = %v", err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "infra.yaml"))
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != want {
		t.Errorf("findConfig() = %v, want %v", got, want)
	}
}

func TestLoadConfig_ProjectRootDefaultsToConfigDir(t *testing.T) {
	// Given infra.yaml outside the current directory
	dir := t.TempDir()
	configPath := filepath.Join(dir, "infra.yaml")
	yamlContent := `
envs:
  prod: {hosted_on: gcp, infra_dir: terraform}
backend_buckets:
  prod: {hosted_on: GCP, bucket_name: b}
tfvars:
  prod: {}
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// When it is loaded
	config, err := loadConfig(configPath)

	// Then infra_dir is relative to the config file, and hosted_on is case-insensitive
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if config.ProjectRoot != dir {
		t.Errorf("ProjectRoot = %v, want %v", config.ProjectRoot, dir)
	}
	if config.Envs["prod"].HostedOn != GCP {
		t.Errorf("HostedOn = %v, want GCP", config.Envs["prod"].HostedOn)
	}
}

func TestValidate_ReportsAllErrors(t *testing.T) {
	config := Config{
		Envs: map[string]Env{
			"prod": {HostedOn: "GCP"},
		},
		BackendBuckets: map[string]BackendBucket{
			"prod":  {HostedOn: "GCP"},
			"stray": {HostedOn: "GCP", BucketName: "b"},
		},
		Tfvars: map[string]map[string]interface{}{
			"prod":  {},
			"other": {},
		},
	}

	err := config.validate()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"missing infra_dir", "no bucket_name", `"stray" which is not defined`, `"other" which is not defined`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveEnv(t *testing.T) {
	config := Config{Envs: map[string]Env{"prod": {}, "Dev": {}}}

	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"prod", "prod", false},
		{"PROD", "prod", false},
		{"dev", "Dev", false},
		{"staging", "", true},
	}
	for _, tt := range tests {
		got, err := config.resolveEnv(tt.in)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("resolveEnv(%q) = %q, %v; want %q, error=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}
