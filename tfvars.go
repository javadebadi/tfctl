package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func writeTfvars(env string, infraDir string, vars map[string]interface{}) (err error) {
	if err := os.MkdirAll(infraDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(infraDir, getEnvVarsFileName(env))

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err := f.Write(hclwrite.Format([]byte(formatTfvars(vars)))); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "wrote", path)
	return nil
}

// formatTfvars renders vars as the content of a .tfvars file, keys sorted.
// The result is not aligned; hclwrite.Format does that like terraform fmt.
func formatTfvars(vars map[string]interface{}) string {
	var b strings.Builder
	for _, key := range sortedKeys(vars) {
		fmt.Fprintf(&b, "%s = %s\n", key, formatHCLValue(vars[key], ""))
	}
	return b.String()
}

// formatHCLValue renders a value decoded from YAML as an HCL expression
func formatHCLValue(value interface{}, indent string) string {
	inner := indent + "  "
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return quoteHCLString(v)
	case time.Time:
		return quoteHCLString(v.Format(time.RFC3339))
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		var b strings.Builder
		b.WriteString("[\n")
		for _, item := range v {
			fmt.Fprintf(&b, "%s%s,\n", inner, formatHCLValue(item, inner))
		}
		b.WriteString(indent + "]")
		return b.String()
	case map[string]interface{}:
		if len(v) == 0 {
			return "{}"
		}
		var b strings.Builder
		b.WriteString("{\n")
		for _, key := range sortedKeys(v) {
			fmt.Fprintf(&b, "%s%s = %s\n", inner, quoteHCLString(key), formatHCLValue(v[key], inner))
		}
		b.WriteString(indent + "}")
		return b.String()
	case map[interface{}]interface{}:
		converted := make(map[string]interface{}, len(v))
		for key, item := range v {
			converted[fmt.Sprint(key)] = item
		}
		return formatHCLValue(converted, indent)
	default:
		return quoteHCLString(fmt.Sprint(v))
	}
}

// quoteHCLString quotes s as an HCL string literal. Template sequences
// ("${" and "%{") are escaped so values are taken literally.
func quoteHCLString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '$', '%':
			b.WriteRune(r)
			if i+1 < len(s) && s[i+1] == '{' {
				b.WriteRune(r)
			}
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// parseVariables reads every *.tf file in infraDir (not recursive, like
// Terraform itself) and returns all declared variables and those that have
// a default (i.e. are optional)
func parseVariables(infraDir string) (all map[string]bool, optional map[string]bool, err error) {
	files, err := filepath.Glob(filepath.Join(infraDir, "*.tf"))
	if err != nil {
		return nil, nil, err
	}

	all = map[string]bool{}
	optional = map[string]bool{}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, err
		}
		parsed, diags := hclsyntax.ParseConfig(src, file, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			return nil, nil, diags
		}
		body, ok := parsed.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range body.Blocks {
			if block.Type != "variable" || len(block.Labels) != 1 {
				continue
			}
			name := block.Labels[0]
			all[name] = true
			if _, hasDefault := block.Body.Attributes["default"]; hasDefault {
				optional[name] = true
			}
		}
	}
	return all, optional, nil
}

// validateTfvars checks the tfvars for env against the variables declared in
// infraDir: every variable without a default must be set, and every key set
// must be declared
func validateTfvars(env string, infraDir string, vars map[string]interface{}) error {
	all, optional, err := parseVariables(infraDir)
	if err != nil {
		return fmt.Errorf("reading terraform variables in %s: %w", infraDir, err)
	}

	var missing, extra []string
	for name := range all {
		if _, set := vars[name]; !set && !optional[name] {
			missing = append(missing, name)
		}
	}
	for name := range vars {
		if !all[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	var errs []error
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("env %q: missing required variables: %s", env, strings.Join(missing, ", ")))
	}
	if len(extra) > 0 {
		errs = append(errs, fmt.Errorf("env %q: keys not declared as variables in %s: %s", env, infraDir, strings.Join(extra, ", ")))
	}
	if len(errs) > 0 {
		return fmt.Errorf("tfvars do not match terraform variables:\n%w", errors.Join(errs...))
	}
	return nil
}
