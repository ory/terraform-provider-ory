package acctest

import (
	"os"
	"strings"
	"testing"
	"text/template"
)

// LoadTestConfig loads a Terraform test configuration from a testdata file.
// The path is relative to the test's working directory (e.g., "testdata/basic.tf").
//
// If data is non-nil, the file is parsed as a Go template using [[ ]] delimiters
// to avoid conflicts with Terraform's {{ }} syntax. Template variables use
// [[ .VarName ]] in the .tf file.
//
// The provider block is automatically prepended.
func LoadTestConfig(t *testing.T, path string, data any) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test config %s: %v", path, err)
	}

	var body string
	if data != nil {
		tmpl, err := template.New(path).Delims("[[", "]]").Parse(string(content))
		if err != nil {
			t.Fatalf("failed to parse test config template %s: %v", path, err)
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, data); err != nil {
			t.Fatalf("failed to render test config %s: %v", path, err)
		}
		body = buf.String()
	} else {
		body = string(content)
	}

	return "provider \"ory\" {}\n\n" + body
}
