package tools

import "sort"

// CLIPreset defines a built-in configuration template for a common CLI tool.
// Presets eliminate admin research friction by pre-filling env var names,
// deny patterns, timeout, and usage tips.
type CLIPreset struct {
	BinaryName  string      `json:"binary_name"`
	Description string      `json:"description"`
	EnvVars     []EnvVarDef `json:"env_vars"`
	DenyArgs    []string    `json:"deny_args"`
	DenyVerbose []string    `json:"deny_verbose"`
	Timeout     int         `json:"timeout"`
	Tips        string      `json:"tips"`
}

// EnvVarDef describes an environment variable required by a CLI tool.
type EnvVarDef struct {
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	IsFile   bool   `json:"is_file,omitempty"`   // credential is a file path (e.g. GOOGLE_APPLICATION_CREDENTIALS)
	Optional bool   `json:"optional,omitempty"`
	Url      string `json:"url,omitempty"`       // URL to navigate and generate the token
}

// CLIPresets contains built-in presets for common CLI tools.
var CLIPresets = map[string]CLIPreset{
	"gh": {
		BinaryName:  "gh",
		Description: "GitHub CLI",
		EnvVars:     []EnvVarDef{{Name: "GH_TOKEN", Desc: "GitHub PAT or App token", Url: "https://github.com/settings/tokens/new"}},
		DenyArgs:    []string{`auth\s+`, `ssh-key`, `gpg-key`, `repo\s+delete`, `secret\s+`},
		DenyVerbose: []string{`--verbose`, `-v`},
		Timeout:     30,
		Tips:        "Use --json flag for structured output",
	},
	"gcloud": {
		BinaryName:  "gcloud",
		Description: "Google Cloud CLI",
		EnvVars: []EnvVarDef{
			{Name: "GOOGLE_APPLICATION_CREDENTIALS", Desc: "Service account JSON (Select Project -> IAM -> Service Accounts)", IsFile: true, Url: "https://console.cloud.google.com/iam-admin/serviceaccounts"},
		},
		DenyArgs:    []string{`iam\s+`, `auth\s+`, `projects\s+delete`, `services\s+disable`, `kms\s+`},
		DenyVerbose: []string{`--verbosity=debug`, `--log-http`},
		Timeout:     120,
		Tips:        "Use --format=json for structured output",
	},
	"aws": {
		BinaryName:  "aws",
		Description: "AWS CLI",
		EnvVars: []EnvVarDef{
			{Name: "AWS_ACCESS_KEY_ID", Desc: "AWS access key", Url: "https://console.aws.amazon.com/iam/home#/security_credentials"},
			{Name: "AWS_SECRET_ACCESS_KEY", Desc: "AWS secret key"},
			{Name: "AWS_DEFAULT_REGION", Desc: "AWS region", Optional: true},
		},
		DenyArgs:    []string{`iam\s+`, `organizations\s+`, `sts\s+assume`, `ec2\s+terminate`},
		DenyVerbose: []string{`--debug`},
		Timeout:     60,
		Tips:        "Use --output json for structured output",
	},
	"kubectl": {
		BinaryName:  "kubectl",
		Description: "Kubernetes CLI",
		EnvVars: []EnvVarDef{
			{Name: "KUBECONFIG", Desc: "Path to kubeconfig", IsFile: true},
		},
		DenyArgs:    []string{`delete\s+namespace`, `delete\s+node`, `drain\s+`, `cordon\s+`},
		DenyVerbose: nil,
		Timeout:     60,
		Tips:        "Use -o json for structured output",
	},
	"terraform": {
		BinaryName:  "terraform",
		Description: "Terraform CLI",
		EnvVars: []EnvVarDef{
			{Name: "TF_TOKEN_app_terraform_io", Desc: "Terraform Cloud token", Optional: true, Url: "https://app.terraform.io/app/settings/tokens"},
		},
		DenyArgs:    []string{`destroy`, `force-unlock`},
		DenyVerbose: nil,
		Timeout:     300,
		Tips:        "Use -json flag for structured output",
	},
	"az": {
		BinaryName:  "az",
		Description: "Azure CLI",
		EnvVars: []EnvVarDef{
			{Name: "AZURE_DEVOPS_EXT_PAT", Desc: "Azure DevOps PAT (Go to your Organization -> User Settings)", Url: "https://dev.azure.com"},
		},
		DenyArgs: []string{
			`login`, `logout`, `account\s+(clear|set)`, `ad\s+`, `role\s+assignment\s+(create|delete)`,
			`keyvault\s+(delete|purge)`, `lock\s+delete`, `vm\s+delete`, `aks\s+delete`, `group\s+delete`,
			`configure\s+--defaults`,
		},
		DenyVerbose: []string{`--debug`},
		Timeout:     60,
		Tips:        "Use --output json for structured output. PAT authenticates az devops, az repos, and az pipelines commands only.",
	},
}

// GetPreset returns a preset by name, or nil if not found.
func GetPreset(name string) *CLIPreset {
	p, ok := CLIPresets[name]
	if !ok {
		return nil
	}
	return &p
}

// ListPresetNames returns all available preset names sorted alphabetically.
func ListPresetNames() []string {
	names := make([]string, 0, len(CLIPresets))
	for name := range CLIPresets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
