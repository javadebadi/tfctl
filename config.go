package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// defaultConfigName is the file searched for (cwd, then each parent) when --config is not given
const defaultConfigName = "infra.yaml"

// HostedOn restricts which cloud providers are valid
type HostedOn string

const (
	AWS HostedOn = "AWS"
	GCP HostedOn = "GCP"
)

// isValidHostedOn checks whether this HostedOn value is one of the known providers
func (h HostedOn) isValidHostedOn() bool {
	switch h {
	case AWS, GCP:
		return true
	}
	return false
}

// UnmarshalYAML accepts hosted_on in any case (gcp, Gcp, GCP)
func (h *HostedOn) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	*h = HostedOn(strings.ToUpper(s))
	return nil
}

type Env struct {
	HostedOn     HostedOn `yaml:"hosted_on"`
	AWSProfile   string   `yaml:"aws_profile"`
	GCPProjectID string   `yaml:"gcp_project_id"`
	InfraDir     string   `yaml:"infra_dir"`
}

type BackendBucket struct {
	HostedOn   HostedOn `yaml:"hosted_on"`
	BucketName string   `yaml:"bucket_name"`
	Region     string   `yaml:"region"`
}

type Config struct {
	// ProjectRoot is the directory infra_dir paths are relative to.
	// Defaults to the directory containing the config file.
	ProjectRoot    string                            `yaml:"project_root"`
	Envs           map[string]Env                    `yaml:"envs"`
	BackendBuckets map[string]BackendBucket          `yaml:"backend_buckets"`
	Tfvars         map[string]map[string]interface{} `yaml:"tfvars"`
}

// findConfig walks up from dir looking for infra.yaml
func findConfig(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, defaultConfigName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find %s in the current directory or any parent; pass --config", defaultConfigName)
		}
		dir = parent
	}
}

// loadConfig reads, parses, and validates the YAML config file at the given path
func loadConfig(path string) (Config, error) {
	var config Config

	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return config, err
	}
	configDir := filepath.Dir(absPath)
	if config.ProjectRoot == "" {
		config.ProjectRoot = configDir
	} else if !filepath.IsAbs(config.ProjectRoot) {
		config.ProjectRoot = filepath.Join(configDir, config.ProjectRoot)
	}

	if err := config.validate(); err != nil {
		return config, fmt.Errorf("invalid %s:\n%w", path, err)
	}

	return config, nil
}

// validate checks the structure of the config and reports every problem at once
func (c Config) validate() error {
	return errors.Join(append(c.validateEnvs(), c.validateBackendBuckets()...)...)
}

func (c Config) validateEnvs() []error {
	if len(c.Envs) == 0 {
		return []error{fmt.Errorf("config has no environments defined under 'envs'")}
	}

	var errs []error
	for _, name := range sortedKeys(c.Envs) {
		env := c.Envs[name]
		if env.InfraDir == "" {
			errs = append(errs, fmt.Errorf("env %q is missing infra_dir", name))
		}
		if !env.HostedOn.isValidHostedOn() {
			errs = append(errs, fmt.Errorf("env %q has invalid hosted_on %q (must be AWS or GCP)", name, env.HostedOn))
		}

		if _, ok := c.BackendBuckets[name]; !ok {
			errs = append(errs, fmt.Errorf("env %q has no matching entry in backend_buckets", name))
		}
		if _, ok := c.Tfvars[name]; !ok {
			errs = append(errs, fmt.Errorf("env %q has no matching entry in tfvars", name))
		}
	}

	for _, name := range sortedKeys(c.Tfvars) {
		if _, ok := c.Envs[name]; !ok {
			errs = append(errs, fmt.Errorf("tfvars has entry %q which is not defined under 'envs'", name))
		}
	}

	return errs
}

func (c Config) validateBackendBuckets() []error {
	var errs []error
	for _, name := range sortedKeys(c.BackendBuckets) {
		bucketInfo := c.BackendBuckets[name]
		if bucketInfo.BucketName == "" {
			errs = append(errs, fmt.Errorf("backend bucket for env %q has no bucket_name", name))
		}
		if !bucketInfo.HostedOn.isValidHostedOn() {
			errs = append(errs, fmt.Errorf("backend bucket for env %q has invalid hosted_on %q", name, bucketInfo.HostedOn))
		}
		if _, ok := c.Envs[name]; !ok {
			errs = append(errs, fmt.Errorf("backend_buckets has entry %q which is not defined under 'envs'", name))
		}
	}
	return errs
}

// resolveEnv returns the env key matching name, ignoring case (so --env PROD finds "prod")
func (c Config) resolveEnv(name string) (string, error) {
	if _, ok := c.Envs[name]; ok {
		return name, nil
	}
	for key := range c.Envs {
		if strings.EqualFold(key, name) {
			return key, nil
		}
	}
	return "", fmt.Errorf("unknown environment %q (available: %s)", name, strings.Join(sortedKeys(c.Envs), ", "))
}

func (c Config) getBackendBucket(env string) (string, error) {
	bucketInfo, ok := c.BackendBuckets[env]
	if !ok {
		return "", fmt.Errorf("no bucket info for env %v", env)
	}
	return bucketInfo.BucketName, nil

}

func (c Config) getInfraDir(env string) (string, error) {
	infraDir := c.Envs[env].InfraDir
	if c.ProjectRoot != "" {
		infraDir = filepath.Join(c.ProjectRoot, c.Envs[env].InfraDir)
	}

	if _, err := os.Stat(infraDir); os.IsNotExist(err) {
		return "", fmt.Errorf("infra_dir %q does not exist", infraDir)
	}

	return infraDir, nil
}

// infraDirs returns every distinct infra dir across all envs, in a stable order
func (c Config) infraDirs() ([]string, error) {
	seen := map[string]bool{}
	var dirs []string
	for _, name := range sortedKeys(c.Envs) {
		dir, err := c.getInfraDir(name)
		if err != nil {
			return nil, err
		}
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
