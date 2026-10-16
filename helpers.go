package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// command describes an external command to run
type command struct {
	name   string
	args   []string
	dir    string    // working directory; empty means the current one
	env    []string  // extra KEY=VALUE pairs on top of the current environment
	stdout io.Writer // defaults to os.Stdout
}

func (c command) run() error {
	cmd := exec.Command(c.name, c.args...)
	cmd.Dir = c.dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = c.stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}
	return cmd.Run()
}

// runCommand runs an external command in the current working directory
func runCommand(name string, args ...string) error {
	return command{name: name, args: args}.run()
}

// runCommandCaptured runs an external command and returns its combined
// stdout and stderr instead of printing them
func runCommandCaptured(name string, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// terraformEnv returns the environment variables terraform needs for env
func terraformEnv(c Config, env string) []string {
	envConfig := c.Envs[env]
	if envConfig.HostedOn == AWS && envConfig.AWSProfile != "" {
		return []string{"AWS_PROFILE=" + envConfig.AWSProfile}
	}
	return nil
}

// runTerraform runs a terraform subcommand (init, plan, apply...) with the
// given extra args, inside the environment's infra dir
func runTerraform(c Config, env string, subcommand string, extraArgs ...string) error {
	return runTerraformTo(c, env, os.Stdout, subcommand, extraArgs...)
}

// runTerraformTo is runTerraform with terraform's stdout sent to stdout
func runTerraformTo(c Config, env string, stdout io.Writer, subcommand string, extraArgs ...string) error {
	infraDir, err := c.getInfraDir(env)
	if err != nil {
		return err
	}

	args := append(strings.Fields(subcommand), extraArgs...)
	return command{
		name:   "terraform",
		args:   args,
		dir:    infraDir,
		env:    terraformEnv(c, env),
		stdout: stdout,
	}.run()
}

func envToLower(env string) string {
	return strings.ToLower(env)
}

func getEnvVarsFileName(env string) string {
	// returns filename of envvars
	return fmt.Sprintf("%s.tfvars", envToLower(env))
}

func varFileArg(env string) string {
	return fmt.Sprintf("-var-file=%s", getEnvVarsFileName(env))
}
