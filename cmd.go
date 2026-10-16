package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	envName    string
	configPath string
	config     Config
)

var rootCmd = &cobra.Command{
	Use:   "tfctl",
	Short: "A CLI to make working with Terraform environments easier",
	Long: `tfctl runs terraform for the environments described in an infra.yaml file.

Before every command that takes --env, the <env>.tfvars file is regenerated
from the tfvars section of infra.yaml (after checking it against the variable
blocks of the terraform code), and terraform init is run with the env's
backend bucket. Never edit <env>.tfvars by hand.

infra.yaml is found by searching the current directory and its parents,
unless --config is given.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,

	// PersistentPreRunE runs before this command's Run, AND before any child
	// subcommand's Run (plan, apply, etc.), so the config is always loaded.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if skipsConfig(cmd) {
			return nil
		}

		path, err := resolveConfigPath()
		if err != nil {
			return err
		}
		config, err = loadConfig(path)
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		return nil
	},
}

// skipsConfig reports whether cmd (or one of its parents) works without infra.yaml
func skipsConfig(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" || c.Name() == "help" || strings.HasPrefix(c.Name(), "__complete") {
			return true
		}
	}
	return false
}

func resolveConfigPath() (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	return findConfig(".")
}

// prepareEnv checks --env, then regenerates <env>.tfvars so it is always fresh
// before terraform ever sees it
func prepareEnv() (string, error) {
	if envName == "" {
		return "", fmt.Errorf("--env is required (available: %s)", strings.Join(sortedKeys(config.Envs), ", "))
	}

	env, err := config.resolveEnv(envName)
	if err != nil {
		return "", err
	}

	infraDir, err := config.getInfraDir(env)
	if err != nil {
		return "", err
	}

	if err := validateTfvars(env, infraDir, config.Tfvars[env]); err != nil {
		return "", err
	}

	if err := writeTfvars(env, infraDir, config.Tfvars[env]); err != nil {
		return "", fmt.Errorf("error writing tfvars: %w", err)
	}
	return env, nil
}

// envCommand builds a command that needs --env. When withInit is set,
// terraform init runs before run.
func envCommand(cmd *cobra.Command, withInit bool, run func(env string, args []string) error) *cobra.Command {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		env, err := prepareEnv()
		if err != nil {
			return err
		}
		if withInit {
			if err := tfInit(config, env); err != nil {
				return fmt.Errorf("terraform init failed: %w", err)
			}
		}
		return run(env, args)
	}
	return cmd
}

// terraformCommand builds a command that runs `terraform <subcommand>` after
// init, with fixedArgs and then the command's own positional args
func terraformCommand(cmd *cobra.Command, subcommand string, fixedArgs func(env string) []string) *cobra.Command {
	return envCommand(cmd, true, func(env string, args []string) error {
		var allArgs []string
		if fixedArgs != nil {
			allArgs = fixedArgs(env)
		}
		return runTerraform(config, env, subcommand, append(allArgs, args...)...)
	})
}

// withVarFile passes -var-file=<env>.tfvars
func withVarFile(env string) []string {
	return []string{varFileArg(env)}
}

const passthroughHelp = "\n\nArguments after -- are passed to terraform as is, e.g.\n  tfctl %s --env prod -- -target=module.foo"

var tfvarsCmd = envCommand(&cobra.Command{
	Use:   "tfvars",
	Short: "Generate a .tfvars file for the given environment",
	Args:  cobra.NoArgs,
}, false, func(env string, args []string) error {
	// nothing to do here: prepareEnv already wrote the file
	return nil
})

var createBackendBucketCmd = envCommand(&cobra.Command{
	Use:   "create-backend-bucket",
	Short: "Create the bucket that stores the terraform state of the given environment",
	Long: `Create the bucket that stores the terraform state of the given environment.

Run this once, before the first init/plan/apply of an environment.`,
	Args: cobra.NoArgs,
}, false, func(env string, args []string) error {
	if err := createBackendBucket(config, env); err != nil {
		return fmt.Errorf("creating backend bucket: %w", err)
	}
	fmt.Println("backend bucket created for env", env)
	return nil
})

var createBackendBucketIfNotExistsCmd = envCommand(&cobra.Command{
	Use:   "create-backend-bucket-if-not-exists",
	Short: "Create the terraform state bucket of the given environment, unless it already exists",
	Args:  cobra.NoArgs,
}, false, func(env string, args []string) error {
	return createBackendBucketIfNotExists(config, env)
})

var backendBucketNameCmd = envCommand(&cobra.Command{
	Use:     "backend-bucket-name",
	Aliases: []string{"get-backend-bucket-name"},
	Short:   "Print the name of the terraform state bucket of the given environment",
	Args:    cobra.NoArgs,
}, false, func(env string, args []string) error {
	name, err := config.getBackendBucket(env)
	if err != nil {
		return err
	}
	fmt.Println(name)
	return nil
})

var setCloudProviderCmd = envCommand(&cobra.Command{
	Use:   "set-cloud-provider",
	Short: "Point the cloud CLI at the given environment (gcloud project, or check the AWS profile)",
	Args:  cobra.NoArgs,
}, false, func(env string, args []string) error {
	return configureCloudProvider(config, env)
})

var tfInitCmd = envCommand(&cobra.Command{
	Use:   "init",
	Short: "Runs terraform init for a given environment",
	Long: `Runs terraform init with the backend config of the given environment.

Every other terraform command runs this first, so you rarely need it directly.

If it fails with "bucket doesn't exist" (or "NoSuchBucket"), run
tfctl create-backend-bucket first.`,
	Args: cobra.NoArgs,
}, false, func(env string, args []string) error {
	return tfInit(config, env)
})

var tfValidateCmd = terraformCommand(&cobra.Command{
	Use:   "validate",
	Short: "Runs terraform validate",
	Args:  cobra.NoArgs,
}, "validate", nil)

var tfPlanCmd = terraformCommand(&cobra.Command{
	Use:   "plan [-- terraform args]",
	Short: "Runs terraform plan for a given environment",
	Long:  "Runs terraform plan for a given environment." + fmt.Sprintf(passthroughHelp, "plan"),
}, "plan", func(env string) []string {
	return []string{"-input=false", varFileArg(env)}
})

var autoApprove bool

func autoApproveArgs(env string) []string {
	args := withVarFile(env)
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	return args
}

var tfApplyCmd = terraformCommand(&cobra.Command{
	Use:   "apply [-- terraform args]",
	Short: "Runs terraform apply for a given environment",
	Long:  "Runs terraform apply for a given environment. It asks for approval unless --auto-approve is set." + fmt.Sprintf(passthroughHelp, "apply"),
}, "apply", autoApproveArgs)

var tfDestroyCmd = terraformCommand(&cobra.Command{
	Use:   "destroy [-- terraform args]",
	Short: "Runs terraform destroy for a given environment",
	Long:  "Runs terraform destroy for a given environment. It asks for approval unless --auto-approve is set." + fmt.Sprintf(passthroughHelp, "destroy"),
}, "destroy", autoApproveArgs)

var tfRefreshCmd = terraformCommand(&cobra.Command{
	Use:   "refresh [-- terraform args]",
	Short: "Updates the state to match the real infrastructure",
}, "refresh", withVarFile)

var (
	outputJSON bool
	outputRaw  bool
)

var tfOutputCmd = terraformCommand(&cobra.Command{
	Use:   "output [NAME]",
	Short: "Shows terraform outputs (all of them, or the one named)",
	Long: `Shows terraform outputs (all of them, or the one named).

terraform init output goes to stderr, so stdout can be captured:
  URL=$(tfctl output --env prod --raw backend_url)`,
	Args: cobra.MaximumNArgs(1),
}, "output", func(env string) []string {
	var args []string
	if outputJSON {
		args = append(args, "-json")
	}
	if outputRaw {
		args = append(args, "-raw")
	}
	return args
})

var tfRawOutputCmd = terraformCommand(&cobra.Command{
	Use:   "raw-output NAME",
	Short: "Prints the raw value of one terraform output (same as output --raw NAME)",
	Long: `Prints the raw value of one terraform output (same as output --raw NAME).

terraform init output goes to stderr, so stdout can be captured:
  URL=$(tfctl raw-output --env prod backend_url)`,
	Args: cobra.ExactArgs(1),
}, "output", func(env string) []string {
	return []string{"-raw"}
})

var showJSON bool

var tfShowCmd = terraformCommand(&cobra.Command{
	Use:   "show",
	Short: "Shows the full terraform state",
	Args:  cobra.NoArgs,
}, "show", func(env string) []string {
	if showJSON {
		return []string{"-json"}
	}
	return nil
})

var tfImportCmd = terraformCommand(&cobra.Command{
	Use:   "import ADDRESS ID",
	Short: "Imports an existing cloud resource into terraform state",
	Long: `Imports an existing cloud resource into terraform state, e.g.
  tfctl import --env prod aws_s3_bucket.website my-bucket-name`,
	Args: cobra.ExactArgs(2),
}, "import", withVarFile)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Inspects and edits terraform state",
}

var stateListCmd = terraformCommand(&cobra.Command{
	Use:   "list [ADDRESS...]",
	Short: "Lists the resources in the state",
}, "state list", nil)

var stateShowCmd = terraformCommand(&cobra.Command{
	Use:   "show ADDRESS",
	Short: "Shows every attribute of one resource in the state",
	Args:  cobra.ExactArgs(1),
}, "state show", nil)

var stateMvCmd = terraformCommand(&cobra.Command{
	Use:   "mv SOURCE DESTINATION",
	Short: "Moves a resource to another address in the state",
	Args:  cobra.ExactArgs(2),
}, "state mv", nil)

var stateRmCmd = terraformCommand(&cobra.Command{
	Use:     "rm ADDRESS...",
	Aliases: []string{"remove"},
	Short:   "Removes resources from the state (they are not destroyed)",
	Args:    cobra.MinimumNArgs(1),
}, "state rm", nil)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manages terraform workspaces",
}

var workspaceListCmd = terraformCommand(&cobra.Command{
	Use:   "list",
	Short: "Lists the workspaces",
	Args:  cobra.NoArgs,
}, "workspace list", nil)

var workspaceShowCmd = terraformCommand(&cobra.Command{
	Use:   "show",
	Short: "Shows the name of the current workspace",
	Args:  cobra.NoArgs,
}, "workspace show", nil)

var workspaceNewCmd = terraformCommand(&cobra.Command{
	Use:   "new NAME",
	Short: "Creates a workspace",
	Args:  cobra.ExactArgs(1),
}, "workspace new", nil)

var workspaceSelectCmd = terraformCommand(&cobra.Command{
	Use:   "select NAME",
	Short: "Selects a workspace",
	Args:  cobra.ExactArgs(1),
}, "workspace select", nil)

var workspaceDeleteCmd = terraformCommand(&cobra.Command{
	Use:   "delete NAME",
	Short: "Deletes a workspace",
	Args:  cobra.ExactArgs(1),
}, "workspace delete", nil)

var tfProvidersCmd = terraformCommand(&cobra.Command{
	Use:   "providers",
	Short: "Shows the required providers and their version constraints",
	Args:  cobra.NoArgs,
}, "providers", nil)

var tfGraphCmd = terraformCommand(&cobra.Command{
	Use:   "graph",
	Short: "Outputs the dependency graph in DOT format (pipe to graphviz: | dot -Tsvg > graph.svg)",
	Args:  cobra.NoArgs,
}, "graph", nil)

var tfConsoleCmd = terraformCommand(&cobra.Command{
	Use:   "console",
	Short: "Opens an interactive console for evaluating terraform expressions",
	Args:  cobra.NoArgs,
}, "console", withVarFile)

var tfForceUnlockCmd = terraformCommand(&cobra.Command{
	Use:   "force-unlock LOCK_ID",
	Short: "Releases a stuck state lock (the lock id is in the error message)",
	Args:  cobra.ExactArgs(1),
}, "force-unlock", func(env string) []string {
	return []string{"-force"}
})

var tfGetCmd = envCommand(&cobra.Command{
	Use:   "get",
	Short: "Downloads and updates modules without reinitialising the backend",
	Args:  cobra.NoArgs,
}, false, func(env string, args []string) error {
	return tfGet(config, env)
})

var tfCmd = terraformCommand(&cobra.Command{
	Use:   "tf -- SUBCOMMAND [ARGS...]",
	Short: "Runs any terraform command in the environment's infra dir, after init",
	Long: `Runs any terraform command in the environment's infra dir, after init.
For anything the other commands do not cover, e.g.
  tfctl tf --env prod -- state pull
  tfctl tf --env prod -- plan -var-file=prod.tfvars -refresh-only`,
	Args: cobra.MinimumNArgs(1),
}, "", nil)

var fmtCheck bool

var tfFmtCmd = &cobra.Command{
	Use:     "fmt",
	Aliases: []string{"autoformat"},
	Short:   "Runs terraform fmt on the infra dir of every environment (or just --env)",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFmt(fmtCheck)
	},
}

var tfCheckFmtCmd = &cobra.Command{
	Use:   "check-autoformat",
	Short: "Checks the terraform files are formatted (fails if not; for CI). Same as fmt --check",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFmt(true)
	},
}

func runFmt(check bool) error {
	var dirs []string
	if envName != "" {
		env, err := config.resolveEnv(envName)
		if err != nil {
			return err
		}
		dir, err := config.getInfraDir(env)
		if err != nil {
			return err
		}
		dirs = []string{dir}
	} else {
		var err error
		if dirs, err = config.infraDirs(); err != nil {
			return err
		}
	}

	for _, dir := range dirs {
		if err := tfFmt(dir, check); err != nil {
			return err
		}
	}
	return nil
}

var validateConfigCmd = &cobra.Command{
	Use:     "validate-config",
	Aliases: []string{"validate-yaml-config"},
	Short:   "Validates infra.yaml, including its tfvars against the terraform variables of every environment",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// the structure was already validated when the config was loaded
		var errs []error
		for _, env := range sortedKeys(config.Envs) {
			infraDir, err := config.getInfraDir(env)
			if err != nil {
				errs = append(errs, fmt.Errorf("env %q: %w", env, err))
				continue
			}
			if err := validateTfvars(env, infraDir, config.Tfvars[env]); err != nil {
				errs = append(errs, err)
			}
		}
		if err := errors.Join(errs...); err != nil {
			return fmt.Errorf("infra.yaml validation failed:\n%w", err)
		}
		fmt.Println("infra.yaml is valid.")
		return nil
	},
}

// completeEnvs offers the env names of infra.yaml for --env
func completeEnvs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	path, err := resolveConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	c, err := loadConfig(path)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return sortedKeys(c.Envs), cobra.ShellCompDirectiveNoFileComp
}

func init() {
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "environment name (e.g. prod, staging)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to the config YAML file (default: infra.yaml in the current directory or a parent)")
	_ = rootCmd.RegisterFlagCompletionFunc("env", completeEnvs)

	tfApplyCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "apply without asking for approval")
	tfDestroyCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "destroy without asking for approval")
	tfOutputCmd.Flags().BoolVar(&outputJSON, "json", false, "print in JSON format")
	tfOutputCmd.Flags().BoolVar(&outputRaw, "raw", false, "print the raw value of a single output (no quotes)")
	tfShowCmd.Flags().BoolVar(&showJSON, "json", false, "print in JSON format")
	tfFmtCmd.Flags().BoolVar(&fmtCheck, "check", false, "only check the formatting and fail if a file needs changes (for CI)")

	stateCmd.AddCommand(stateListCmd, stateShowCmd, stateMvCmd, stateRmCmd)
	workspaceCmd.AddCommand(workspaceListCmd, workspaceShowCmd, workspaceNewCmd, workspaceSelectCmd, workspaceDeleteCmd)

	rootCmd.AddCommand(
		tfvarsCmd,
		createBackendBucketCmd,
		createBackendBucketIfNotExistsCmd,
		backendBucketNameCmd,
		setCloudProviderCmd,
		tfInitCmd,
		tfValidateCmd,
		tfPlanCmd,
		tfApplyCmd,
		tfDestroyCmd,
		tfRefreshCmd,
		tfOutputCmd,
		tfRawOutputCmd,
		tfShowCmd,
		tfImportCmd,
		stateCmd,
		workspaceCmd,
		tfProvidersCmd,
		tfGraphCmd,
		tfConsoleCmd,
		tfForceUnlockCmd,
		tfGetCmd,
		tfCmd,
		tfFmtCmd,
		tfCheckFmtCmd,
		validateConfigCmd,
	)
}

// Execute runs the root command — called from main()
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	// terraform (or gcloud/aws) already printed why it failed; exit with its code
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code > 0 {
			os.Exit(code)
		}
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
