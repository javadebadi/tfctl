package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFormatTfvars(t *testing.T) {
	// Given tfvars decoded from YAML, with every kind of value
	var vars map[string]interface{}
	yamlContent := `
name: knaph
count: 3
ratio: 0.5
enabled: true
nothing: null
emails: ["a@b.com", "c@d.com"]
empty_list: []
labels: {team: core, tier: 1}
`
	if err := yaml.Unmarshal([]byte(yamlContent), &vars); err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	// When formatted as tfvars
	got := formatTfvars(vars)

	// Then keys are sorted and every value keeps its type
	want := `count = 3
emails = [
  "a@b.com",
  "c@d.com",
]
empty_list = []
enabled = true
labels = {
  "team" = "core"
  "tier" = 1
}
name = "knaph"
nothing = null
ratio = 0.5
`
	if got != want {
		t.Errorf("formatTfvars() =\n%s\nwant\n%s", got, want)
	}
}

func TestQuoteHCLString(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`plain`, `"plain"`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
		{"two\nlines", `"two\nlines"`},
		{`${var.x}`, `"$${var.x}"`},
		{`%{if x}`, `"%%{if x}"`},
		{`$5 and 100%`, `"$5 and 100%"`},
		{`héllo`, `"héllo"`},
	}
	for _, tt := range tests {
		if got := quoteHCLString(tt.in); got != tt.want {
			t.Errorf("quoteHCLString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestWriteTfvars(t *testing.T) {
	// Given an infra dir
	dir := t.TempDir()

	// When tfvars are written for env PROD
	err := writeTfvars("PROD", dir, map[string]interface{}{"region": "us-central1", "environment": "prod"})

	// Then prod.tfvars exists and is formatted like terraform fmt would
	if err != nil {
		t.Fatalf("writeTfvars() error = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "prod.tfvars"))
	if err != nil {
		t.Fatalf("reading tfvars: %v", err)
	}
	want := "environment = \"prod\"\nregion      = \"us-central1\"\n"
	if string(got) != want {
		t.Errorf("prod.tfvars =\n%s\nwant\n%s", got, want)
	}
}

func writeTfFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func TestParseVariables(t *testing.T) {
	// Given variables spread over several files, with nested blocks,
	// braces inside strings, and a commented out variable
	dir := t.TempDir()
	writeTfFile(t, dir, "variables.tf", `
variable "name" {
  type = string
  validation {
    condition     = length(var.name) > 0
    error_message = "name must not be empty {"
  }
}
# variable "commented" {}
variable "region" {
  default = "us-central1"
}
`)
	writeTfFile(t, dir, "feature.tf", `
variable "optional" { default = null }
resource "null_resource" "x" {}
`)
	writeTfFile(t, dir, "notes.txt", `variable "ignored" {}`)

	// When the variables are parsed
	all, optional, err := parseVariables(dir)

	// Then every variable is found, and those with defaults are optional
	if err != nil {
		t.Fatalf("parseVariables() error = %v", err)
	}
	for _, name := range []string{"name", "region", "optional"} {
		if !all[name] {
			t.Errorf("variable %q not found", name)
		}
	}
	if len(all) != 3 {
		t.Errorf("found %d variables, want 3: %v", len(all), all)
	}
	if optional["name"] || !optional["region"] || !optional["optional"] {
		t.Errorf("wrong optional variables: %v", optional)
	}
}

func TestParseVariables_InvalidHCL(t *testing.T) {
	dir := t.TempDir()
	writeTfFile(t, dir, "broken.tf", `variable "x" {`)

	if _, _, err := parseVariables(dir); err == nil {
		t.Error("expected an error for invalid HCL, got nil")
	}
}

func TestValidateTfvars(t *testing.T) {
	dir := t.TempDir()
	writeTfFile(t, dir, "variables.tf", `
variable "required" {}
variable "optional" { default = 1 }
`)

	tests := []struct {
		name    string
		vars    map[string]interface{}
		wantErr string
	}{
		{"only required", map[string]interface{}{"required": "x"}, ""},
		{"required and optional", map[string]interface{}{"required": "x", "optional": 2}, ""},
		{"missing required", map[string]interface{}{"optional": 2}, "missing required variables: required"},
		{"undeclared key", map[string]interface{}{"required": "x", "typo": 1}, "not declared as variables"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTfvars("prod", dir, tt.vars)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateTfvars() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateTfvars() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
