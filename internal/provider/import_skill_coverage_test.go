package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportSkillCoversAllResources guards the import-existing-project agent
// skill against drift: every resource registered on the provider must appear
// in the skill's resource reference table and be handled by the inventory
// script (either with an emitted import block or an explanatory comment), and
// resources without import support must be documented as not importable.
//
// When this test fails after adding, renaming, or removing a resource, update
// .claude/skills/import-existing-project/ (SKILL.md reference table and
// scripts/generate-imports.sh) to match.
func TestImportSkillCoversAllResources(t *testing.T) {
	ctx := context.Background()

	skillDir := filepath.Join("..", "..", ".claude", "skills", "import-existing-project")
	skillBytes, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	require.NoError(t, err, "the import skill must ship with the provider")
	scriptBytes, err := os.ReadFile(filepath.Join(skillDir, "scripts", "generate-imports.sh"))
	require.NoError(t, err, "the import skill inventory script must ship with the provider")
	skill := string(skillBytes)
	script := string(scriptBytes)

	p := New("test")()

	for _, newResource := range p.(*OryProvider).Resources(ctx) {
		r := newResource()

		var md frameworkresource.MetadataResponse
		r.Metadata(ctx, frameworkresource.MetadataRequest{ProviderTypeName: "ory"}, &md)
		name := md.TypeName
		require.NotEmpty(t, name, "resource returned an empty type name")

		_, importable := r.(frameworkresource.ResourceWithImportState)

		tableRow := findSkillTableRow(skill, name)
		assert.NotEmpty(t, tableRow,
			"%s is missing from the resource reference table in SKILL.md", name)

		assert.Contains(t, script, name,
			"%s is not handled by generate-imports.sh", name)

		if tableRow == "" {
			continue
		}
		if importable {
			assert.NotContains(t, tableRow, "not importable",
				"%s supports import but its SKILL.md table row says otherwise", name)
		} else {
			assert.Contains(t, tableRow, "not importable",
				"%s does not support import; its SKILL.md table row must say so", name)
		}
	}
}

// findSkillTableRow returns the SKILL.md reference-table row for the given
// resource type name, or "" if the table has no row for it.
func findSkillTableRow(skill, resourceName string) string {
	for _, line := range strings.Split(skill, "\n") {
		if strings.HasPrefix(line, fmt.Sprintf("| `%s`", resourceName)) {
			return line
		}
	}
	return ""
}
