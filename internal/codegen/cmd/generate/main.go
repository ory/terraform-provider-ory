// Command generate reads mappings.yaml and produces Go source files for the
// project_config resource: schema_gen.go, patches_gen.go, and read_gen.go.
//
// It optionally reads an OpenAPI spec (--spec) to derive patch paths from
// "governs" descriptions in the normalizedProjectRevision schema, falling
// back to explicit patch_path values in mappings.yaml.
//
//go:generate go run . -mappings ../../mappings.yaml -out ../../../resources/projectconfig/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Attribute struct {
	Name            string `yaml:"name"`
	GoField         string `yaml:"go_field"`
	Type            string `yaml:"type"` // string, bool, int64, list_string, map_string
	PatchPath       string `yaml:"patch_path"`
	OpenAPIProperty string `yaml:"openapi_property"` // maps to normalizedProjectRevision property name
	Description     string `yaml:"description"`
	Computed        bool   `yaml:"computed"`
	DefaultBool     *bool  `yaml:"default_bool"`
	DefaultInt64    *int64 `yaml:"default_int64"`
	Sensitive       bool   `yaml:"sensitive"`
	SkipEmptyRead   bool   `yaml:"skip_empty_read"`
	WriteOnly       bool   `yaml:"write_only"` // omit from the API read path: sent on create/update but never read back into state (for secrets the API does not return, or returns masked)
	// PreserveOnMissing keeps the state value when the API response omits this
	// attribute's key. Set it for attributes the API accepts with HTTP 200 but
	// never reports back, where nulling state would create a diff that no apply
	// can settle. Verify with curl before setting it: PATCH the value, then GET
	// the project and check whether the key is in the response.
	PreserveOnMissing bool `yaml:"preserve_on_missing"`
	// ReadPath is the config path the API reports the value under, when that
	// differs from the path the value is written to. Reads use it instead of
	// PatchPath. Always record the verification in a comment next to the
	// attribute.
	ReadPath string `yaml:"read_path"`
	// StorageURLContent marks a string attribute whose content the Ory API
	// uploads to object storage. The write sends `base64://<payload>` and the
	// API reports an https URL instead, so Read resolves the URL back to the
	// configured value. Only valid for `type: string`.
	StorageURLContent bool `yaml:"storage_url_content"`
	// PatchPathOverride replaces the spec-derived patch path when the spec's
	// governs description names a config key the API does not actually read
	// (the write is accepted with HTTP 200 and silently discarded). Reads use
	// it too unless read_path is also set. Always verify the override against
	// the live API and record the verification in a comment.
	PatchPathOverride string `yaml:"patch_path_override"`
	// BoolEnum marks a bool attribute whose config key stores a string enum
	// instead of a JSON bool. The patch sends true_value/false_value and Read
	// maps the reported string back by comparing it to true_value. Sending a
	// raw bool to such a key is accepted with HTTP 200 but normalized by
	// string comparison, so it silently stores the false variant.
	BoolEnum *BoolEnum `yaml:"bool_enum"`
	// RevisionProperty marks an attribute that is a top-level normalized
	// revision column with no key in any service config document. Document
	// PATCHes cannot reach it (they return HTTP 200 and silently discard the
	// write), so the write goes through the normalized revision endpoint
	// (PATCH /normalized/projects/{id}/revision/{rev}) with op path
	// /<openapi_property>, and the read comes from the normalized revision
	// (GET /normalized/projects/{id}) instead of the project document.
	RevisionProperty bool       `yaml:"revision_property"`
	Validators       *Validator `yaml:"validators"`

	// Deprecated alias support: when set, generates a second schema attribute
	// with the old name that shows a deprecation warning directing users to Name.
	DeprecatedName    string `yaml:"deprecated_name"`     // old terraform attribute name
	DeprecatedGoField string `yaml:"deprecated_go_field"` // old Go struct field name
}

// ReadPreservesOnMissing reports whether Read must keep the value already in
// state when the API response omits this attribute's key, instead of nulling it
// to signal drift.
//
// Three groups qualify. Sensitive attributes are redacted or dropped by the API
// by design. skip_empty_read attributes are the ones the API does not report
// faithfully. preserve_on_missing attributes are the ones the API accepts and
// then never reports, so nulling them would produce a diff no apply can settle.
func (a Attribute) ReadPreservesOnMissing() bool {
	return a.Sensitive || a.SkipEmptyRead || a.PreserveOnMissing
}

// ReadKeysPath returns the config path the generated Read looks the value up
// under. It is PatchPath for almost every attribute, and read_path for the few
// the API reports somewhere other than where it accepts the write.
func (a Attribute) ReadKeysPath() string {
	if a.ReadPath != "" {
		return a.ReadPath
	}
	return a.PatchPath
}

// BoolEnum holds the string values a bool_enum attribute's config key stores
// for true and false (e.g. "password"/"default").
type BoolEnum struct {
	TrueValue  string `yaml:"true_value"`
	FalseValue string `yaml:"false_value"`
}

type Validator struct {
	OneOf        []string `yaml:"one_of"`
	Regex        string   `yaml:"regex"`
	RegexMessage string   `yaml:"regex_message"`
}

type Mappings struct {
	Attributes []Attribute `yaml:"attributes"`
}

type ServiceConfig struct {
	ConfigExpr string
	NilCheck   string
}

var serviceConfigs = map[string]ServiceConfig{
	"identity":           {"project.Services.Identity.Config", "project.Services.Identity != nil"},
	"oauth2":             {"project.Services.Oauth2.Config", "project.Services.Oauth2 != nil"},
	"account_experience": {"project.Services.AccountExperience.Config", "project.Services.AccountExperience != nil"},
	"permission":         {"project.Services.Permission.Config", "project.Services.Permission != nil"},
}

// prefixToService maps OpenAPI property name prefixes to service paths.
// Longer prefixes are checked first to avoid partial matches.
var prefixToService = map[string]string{
	"kratos_":                     "/services/identity/config/",
	"hydra_":                      "/services/oauth2/config/",
	"keto_":                       "/services/permission/config/",
	"account_experience_":         "/services/account_experience/config/",
	"disable_account_experience_": "/services/account_experience/config/",
	"enable_ax_":                  "/services/account_experience/config/",
}

// governsRegex extracts the config path from a "governs" description.
// Matches: This governs the "session.lifespan" setting.
var governsRegex = regexp.MustCompile(`This governs the "([^"]+)" setting`)

// paragraphBreakRegex matches blank-line paragraph separators. Spec descriptions
// frequently split a single field's docs across multiple paragraphs; cleanDescription
// uses this to join them into space-separated sentences without running together.
var paragraphBreakRegex = regexp.MustCompile(`\n[ \t]*\n`)

// courierTemplatePropertyPrefix matches every kratos_courier_templates_* spec
// property. Courier templates are managed by the dedicated `ory_email_template`
// resource and are not viable as simple-string codegen entries: writes require
// `base64://` encoding and reads return a storage URL whose filename is
// sha512(content). Excluding by prefix (rather than enumerating each template
// family) ensures new families added upstream are excluded automatically. See
// issue #213.
const courierTemplatePropertyPrefix = "kratos_courier_templates_"

func parseService(patchPath string) (string, []string) {
	parts := strings.Split(strings.TrimPrefix(patchPath, "/"), "/")
	if len(parts) < 4 || parts[0] != "services" || parts[2] != "config" {
		log.Fatalf("unexpected patch_path format: %s", patchPath)
	}
	return parts[1], parts[3:]
}

// governsToPatchPath derives a JSON Patch path from an OpenAPI property name
// and its "governs" description. Returns ("", false) if not derivable.
func governsToPatchPath(propertyName, description string) (string, bool) {
	matches := governsRegex.FindStringSubmatch(description)
	if len(matches) < 2 {
		return "", false
	}
	governsPath := matches[1]

	// Sort prefixes longest first to avoid partial matches
	// (e.g., "disable_account_experience_" before "account_experience_")
	prefixes := make([]string, 0, len(prefixToService))
	for prefix := range prefixToService {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })

	for _, prefix := range prefixes {
		if strings.HasPrefix(propertyName, prefix) {
			configPath := strings.ReplaceAll(governsPath, ".", "/")
			return prefixToService[prefix] + configPath, true
		}
	}
	return "", false
}

// =============================================================================
// OpenAPI spec parsing
// =============================================================================

// SpecProperty represents a property from the OpenAPI normalizedProjectRevision schema.
type SpecProperty struct {
	Name                     string
	Type                     string // "string", "boolean", "integer", "array", "object"
	ItemType                 string // for arrays: the element type (e.g., "string")
	AdditionalPropertiesType string // for objects with additionalProperties: the value type
	Description              string
	GovernsPath              string // derived patch path from "governs" description, or ""
}

// parseOpenAPISpec reads an OpenAPI spec and extracts normalizedProjectRevision properties.
func parseOpenAPISpec(specPath string) (map[string]SpecProperty, error) {
	cleanPath := filepath.Clean(specPath)
	data, err := os.ReadFile(cleanPath) // #nosec G304 -- specPath is a CLI flag controlled by the developer, not user input
	if err != nil {
		return nil, fmt.Errorf("reading spec: %w", err)
	}

	// Try JSON first, then YAML
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parsing spec (tried JSON and YAML): %w", err)
		}
	}

	// Navigate to components.schemas.normalizedProjectRevision.properties
	schemas := navigateMap(spec, "components", "schemas", "normalizedProjectRevision", "properties")
	if schemas == nil {
		return nil, fmt.Errorf("could not find components.schemas.normalizedProjectRevision.properties in spec")
	}

	propsMap, ok := schemas.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("normalizedProjectRevision.properties is not a map")
	}

	result := make(map[string]SpecProperty, len(propsMap))
	for name, propRaw := range propsMap {
		prop, ok := propRaw.(map[string]interface{})
		if !ok {
			continue
		}

		sp := SpecProperty{Name: name}

		if t, ok := prop["type"].(string); ok {
			sp.Type = t
		}
		if items, ok := prop["items"].(map[string]interface{}); ok {
			if it, ok := items["type"].(string); ok {
				sp.ItemType = it
			}
		}
		if ap, ok := prop["additionalProperties"].(map[string]interface{}); ok {
			if apt, ok := ap["type"].(string); ok {
				sp.AdditionalPropertiesType = apt
			}
		}
		if d, ok := prop["description"].(string); ok {
			sp.Description = d
		}

		// Try to derive patch path from "governs" description
		if sp.Description != "" {
			if path, ok := governsToPatchPath(name, sp.Description); ok {
				sp.GovernsPath = path
			}
		}

		result[name] = sp
	}

	return result, nil
}

func navigateMap(m map[string]interface{}, keys ...string) interface{} {
	var current interface{} = m
	for _, key := range keys {
		cm, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = cm[key]
		if !ok {
			return nil
		}
	}
	return current
}

const (
	typeString     = "string"
	typeBool       = "bool"
	typeInt64      = "int64"
	typeListString = "list_string"
	typeMapString  = "map_string"
)

// openAPITypeToTFType maps OpenAPI types to Terraform types.
func openAPITypeToTFType(oaType string) string {
	switch oaType {
	case "string":
		return typeString
	case "boolean":
		return typeBool
	case "integer":
		return typeInt64
	default:
		return ""
	}
}

// discoverTypeToTFType extends openAPITypeToTFType with array and map support for discover mode.
// For arrays, itemType must be "string" to map to list_string.
// For objects, additionalPropertiesType must be "string" to map to map_string.
// Other element types are unsupported and return "".
func discoverTypeToTFType(oaType, itemType, additionalPropertiesType string) string {
	if t := openAPITypeToTFType(oaType); t != "" {
		return t
	}
	switch {
	case oaType == "array" && itemType == typeString:
		return typeListString
	case oaType == "object" && additionalPropertiesType == typeString:
		return typeMapString
	default:
		return ""
	}
}

func main() {
	mappingsPath := flag.String("mappings", "mappings.yaml", "path to mappings.yaml")
	outDir := flag.String("out", ".", "output directory for generated files")
	specPath := flag.String("spec", "", "path to OpenAPI spec (JSON or YAML) for governs-based path derivation and validation")
	discover := flag.Bool("discover", false, "output YAML entries for unmapped spec properties (requires --spec)")
	discoverApply := flag.Bool("discover-apply", false, "append YAML entries to mappings file and Go struct fields to resource.go for unmapped spec properties (requires --spec)")
	resourceFile := flag.String("resource-file", "", "path to resource.go for -discover-apply struct field insertion (default: <out>/resource.go)")
	strict := flag.Bool("strict", false, "fail if any spec properties are unmapped (use in CI to detect drift)")
	probeAttrs := flag.String("probe-attributes", "", "comma-separated attribute names to probe against the live console API instead of generating. Requires --spec so patch/read paths resolve for entries that rely on governs derivation (entries with an explicit patch_path work without it), plus ORY_WORKSPACE_API_KEY and either ORY_WORKSPACE_ID (throwaway project) or ORY_PROBE_PROJECT_ID (reuse)")
	probeReport := flag.String("probe-report", "", "file to write the probe markdown report to (default: stdout)")
	flag.Parse()

	cleanMappingsPath := filepath.Clean(*mappingsPath)
	data, err := os.ReadFile(cleanMappingsPath) //nolint:gosec // path from trusted CLI flag
	if err != nil {
		log.Fatalf("reading mappings: %v", err)
	}

	var m Mappings
	if err := yaml.Unmarshal(data, &m); err != nil {
		log.Fatalf("parsing mappings: %v", err)
	}

	// Apply patch path overrides before spec resolution so a wrong governs
	// path in the spec neither wins nor trips the mismatch check.
	for i := range m.Attributes {
		if m.Attributes[i].PatchPathOverride != "" {
			m.Attributes[i].PatchPath = m.Attributes[i].PatchPathOverride
		}
	}

	// If spec provided, parse it and use governs-based path derivation
	var specProps map[string]SpecProperty
	if *specPath != "" {
		var err error
		specProps, err = parseOpenAPISpec(*specPath)
		if err != nil {
			log.Fatalf("parsing spec: %v", err)
		}
		fmt.Printf("Parsed %d properties from OpenAPI spec\n", len(specProps))

		// Resolve patch paths from spec for entries with openapi_property
		resolveFromSpec(&m, specProps)

		if *discover {
			discoverNewEntries(m, specProps)
			return
		}

		if *discoverApply {
			resourcePath := *resourceFile
			if resourcePath == "" {
				resourcePath = filepath.Join(*outDir, "resource.go")
			}
			count, err := applyDiscoveredEntries(m, specProps, cleanMappingsPath, resourcePath)
			if err != nil {
				log.Fatalf("discover-apply: %v", err)
			}
			fmt.Printf("discover-apply: %d new attributes appended\n", count)
			return
		}

		// Report unmapped spec properties (candidates for new entries)
		unmappedCount := reportUnmapped(m, specProps)
		if *strict && unmappedCount > 0 {
			log.Fatalf("STRICT MODE: %d unmapped spec properties found. Run 'make discover' to generate YAML entries.", unmappedCount)
		}
	} else if *discover || *discoverApply {
		log.Fatal("--discover / --discover-apply requires --spec")
	} else if *strict {
		log.Fatal("--strict requires --spec")
	}

	// Validate all attributes have required fields
	for i, a := range m.Attributes {
		if a.Name == "" || a.GoField == "" || a.Type == "" || (a.PatchPath == "" && !a.RevisionProperty) {
			log.Fatalf("attribute %d (%s): name, go_field, type, and patch_path are required (patch_path can be derived from spec via openapi_property)", i, a.Name)
		}
		if a.Type != typeString && a.Type != typeBool && a.Type != typeInt64 && a.Type != typeListString && a.Type != typeMapString {
			log.Fatalf("attribute %q: unsupported type %q", a.Name, a.Type)
		}
		if a.Description == "" {
			log.Fatalf("attribute %q: description is required (set it in mappings.yaml or ensure openapi_property points to a spec entry with a description)", a.Name)
		}
		if a.StorageURLContent && a.Type != typeString {
			log.Fatalf("attribute %q: storage_url_content is only supported for type string, got %q", a.Name, a.Type)
		}
		if a.StorageURLContent && a.WriteOnly {
			log.Fatalf("attribute %q: storage_url_content and write_only are mutually exclusive (write_only removes the attribute from the read path)", a.Name)
		}
		if a.BoolEnum != nil {
			if a.Type != typeBool {
				log.Fatalf("attribute %q: bool_enum is only supported for type bool, got %q", a.Name, a.Type)
			}
			if a.BoolEnum.TrueValue == "" || a.BoolEnum.FalseValue == "" {
				log.Fatalf("attribute %q: bool_enum requires both true_value and false_value", a.Name)
			}
			if a.BoolEnum.TrueValue == a.BoolEnum.FalseValue {
				log.Fatalf("attribute %q: bool_enum true_value and false_value must differ", a.Name)
			}
		}
		if a.RevisionProperty {
			if a.Type != typeBool {
				log.Fatalf("attribute %q: revision_property is only supported for type bool, got %q", a.Name, a.Type)
			}
			if a.OpenAPIProperty == "" {
				log.Fatalf("attribute %q: revision_property requires openapi_property (the revision patch op path is derived from it)", a.Name)
			}
			if a.BoolEnum != nil || a.WriteOnly || a.StorageURLContent || a.ReadPath != "" || a.PatchPathOverride != "" || a.DeprecatedName != "" {
				log.Fatalf("attribute %q: revision_property cannot be combined with bool_enum, write_only, storage_url_content, read_path, patch_path_override, or deprecated aliases", a.Name)
			}
		}
	}

	// Probe mode: write sentinel and empty values to the named attributes on a
	// live project and classify what comes back, instead of generating code.
	if *probeAttrs != "" {
		names := strings.Split(*probeAttrs, ",")
		for i := range names {
			names[i] = strings.TrimSpace(names[i])
		}
		if err := probeAttributes(m, names, *probeReport); err != nil {
			log.Fatalf("probing attributes: %v", err)
		}
		return
	}

	// Group by service. Revision properties are not part of any service config
	// document, so they get no read entries and stay out of the groups; the
	// schema and revision patch templates iterate m.Attributes directly.
	byService := make(map[string][]Attribute)
	hasRegex := false
	for _, a := range m.Attributes {
		if a.RevisionProperty {
			continue
		}
		svc, _ := parseService(a.PatchPath)
		byService[svc] = append(byService[svc], a)
		if a.Validators != nil && a.Validators.Regex != "" {
			hasRegex = true
		}
	}

	// Build sorted service names for deterministic template iteration
	serviceNames := make([]string, 0, len(byService))
	for svc := range byService {
		serviceNames = append(serviceNames, svc)
	}
	sort.Strings(serviceNames)

	td := struct {
		Mappings
		ByService    map[string][]Attribute
		ServiceNames []string
		Services     map[string]ServiceConfig
		HasRegex     bool
	}{m, byService, serviceNames, serviceConfigs, hasRegex}

	funcMap := template.FuncMap{
		"readKeys": func(patchPath string) string {
			_, keys := parseService(patchPath)
			quoted := make([]string, len(keys))
			for i, k := range keys {
				quoted[i] = fmt.Sprintf("%q", k)
			}
			return strings.Join(quoted, ", ")
		},
		"varName": func(svc string) string {
			// Convert service name to Go-idiomatic camelCase variable name.
			// "account_experience" -> "accountExperience"
			// "identity" -> "identity" (no underscores)
			parts := strings.Split(svc, "_")
			if len(parts) <= 1 {
				return svc
			}
			var b strings.Builder
			b.WriteString(parts[0])
			for _, p := range parts[1:] {
				if p == "" {
					continue
				}
				b.WriteString(strings.ToUpper(p[:1]) + p[1:])
			}
			return b.String()
		},
		"filterType": func(attrs []Attribute, typ string) []Attribute {
			var result []Attribute
			for _, a := range attrs {
				if a.Type == typ {
					result = append(result, a)
				}
			}
			return result
		},
		"quoteStrings": func(ss []string) string {
			quoted := make([]string, len(ss))
			for i, s := range ss {
				quoted[i] = fmt.Sprintf("%q", s)
			}
			return strings.Join(quoted, ", ")
		},
		"schemaAttr":           buildSchemaAttr,
		"deprecatedSchemaAttr": buildDeprecatedSchemaAttr,
	}

	cleanOutDir := filepath.Clean(*outDir)

	for _, gen := range []struct {
		name string
		tmpl string
	}{
		{"schema_gen.go", schemaTemplate},
		{"patches_gen.go", patchesTemplate},
		{"read_gen.go", readTemplate},
	} {
		tmpl, err := template.New(gen.name).Funcs(funcMap).Parse(gen.tmpl)
		if err != nil {
			log.Fatalf("parsing template %s: %v", gen.name, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, td); err != nil {
			log.Fatalf("executing template %s: %v", gen.name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			_ = os.WriteFile(filepath.Join(cleanOutDir, gen.name), buf.Bytes(), 0644) // #nosec G306 -- generated source files need standard read permissions
			log.Fatalf("formatting %s: %v (unformatted file written)", gen.name, err)
		}

		if err := os.WriteFile(filepath.Join(cleanOutDir, gen.name), formatted, 0644); err != nil { // #nosec G306 -- generated source files need standard read permissions
			log.Fatalf("writing %s: %v", gen.name, err)
		}
	}

	fmt.Printf("Generated %d attributes into %s\n", len(m.Attributes), cleanOutDir)
}

// resolveFromSpec resolves patch paths from the OpenAPI spec for entries that
// have openapi_property set. If the entry has both openapi_property and patch_path,
// it validates they match and fails fatally on mismatch.
func resolveFromSpec(m *Mappings, specProps map[string]SpecProperty) {
	for i := range m.Attributes {
		a := &m.Attributes[i]
		if a.OpenAPIProperty == "" {
			continue
		}

		sp, ok := specProps[a.OpenAPIProperty]
		if !ok {
			log.Printf("WARNING: openapi_property %q not found in spec for attribute %q", a.OpenAPIProperty, a.Name)
			continue
		}

		// Enrich type from spec if not set in YAML
		if a.Type == "" && sp.Type != "" {
			if tfType := openAPITypeToTFType(sp.Type); tfType != "" {
				a.Type = tfType
				fmt.Printf("  Derived type for %q from spec: %s\n", a.Name, tfType)
			}
		}

		// Enrich description from spec if not set in YAML
		if a.Description == "" && sp.Description != "" {
			if desc := cleanDescription(sp.Description); desc != "" {
				a.Description = desc
			}
		}

		// patch_path_override wins over the spec: the governs description names
		// a config key the API does not actually read, so the mismatch with the
		// derived path is intentional.
		if a.PatchPathOverride != "" {
			continue
		}

		// Revision properties are patched at /<openapi_property> on the
		// revision endpoint; the spec's governs path names a config document
		// key that does not exist, so no document patch path is derived.
		if a.RevisionProperty {
			continue
		}

		if sp.GovernsPath == "" {
			if a.PatchPath == "" {
				log.Printf("WARNING: openapi_property %q has no 'governs' description and no explicit patch_path for attribute %q", a.OpenAPIProperty, a.Name)
			}
			continue
		}

		if a.PatchPath == "" {
			a.PatchPath = sp.GovernsPath
			fmt.Printf("  Derived patch_path for %q from spec: %s\n", a.Name, sp.GovernsPath)
		} else if a.PatchPath != sp.GovernsPath {
			log.Fatalf("ERROR: patch_path mismatch for %q: yaml=%s, governs=%s. Remove explicit patch_path and let governs derive it.", a.Name, a.PatchPath, sp.GovernsPath)
		}
	}
}

// reportUnmapped prints spec properties that have "governs" descriptions but
// aren't mapped in mappings.yaml. Returns the count of unmapped properties.
func reportUnmapped(m Mappings, specProps map[string]SpecProperty) int {
	// Build set of mapped openapi_property values
	mapped := make(map[string]bool)
	for _, a := range m.Attributes {
		if a.OpenAPIProperty != "" {
			mapped[a.OpenAPIProperty] = true
		}
	}

	// Also build set of mapped patch_paths to detect properties already covered
	// by explicit patch_path even without openapi_property
	mappedPaths := make(map[string]bool)
	for _, a := range m.Attributes {
		if a.PatchPath != "" {
			mappedPaths[a.PatchPath] = true
		}
	}

	// Exclusion list: properties that are read-only, managed by other resources,
	// or handled by custom code in resource.go under a different terraform name.
	excluded := excludedProperties()

	var unmapped []SpecProperty
	for _, sp := range specProps {
		if isExcludedProperty(sp.Name, excluded) {
			continue
		}
		if mapped[sp.Name] {
			continue
		}
		// Skip if this property's governs path is already covered
		if sp.GovernsPath != "" && mappedPaths[sp.GovernsPath] {
			continue
		}
		// Only report supported types with governs paths
		if sp.GovernsPath != "" && discoverTypeToTFType(sp.Type, sp.ItemType, sp.AdditionalPropertiesType) != "" {
			unmapped = append(unmapped, sp)
		}
	}

	// Sort for deterministic output
	sortSpecProperties(unmapped)

	if len(unmapped) > 0 {
		fmt.Printf("\n=== Unmapped spec properties with 'governs' (candidates for mappings.yaml) ===\n")
		for _, sp := range unmapped {
			fmt.Printf("  %s (%s) -> %s\n", sp.Name, sp.Type, sp.GovernsPath)
		}
		fmt.Printf("  Total: %d unmapped properties\n", len(unmapped))
	}
	return len(unmapped)
}

// =============================================================================
// Discover mode: auto-generate YAML entries for unmapped spec properties
// =============================================================================

// discoverNewEntries outputs YAML entries and Go struct fields for all
// unmapped spec properties that have "governs" descriptions.
func discoverNewEntries(m Mappings, specProps map[string]SpecProperty) {
	// Build sets of already-mapped properties and paths
	mapped := make(map[string]bool)
	for _, a := range m.Attributes {
		if a.OpenAPIProperty != "" {
			mapped[a.OpenAPIProperty] = true
		}
	}
	mappedPaths := make(map[string]bool)
	for _, a := range m.Attributes {
		if a.PatchPath != "" {
			mappedPaths[a.PatchPath] = true
		}
	}

	excluded := excludedProperties()

	// Collect and sort unmapped properties
	var unmapped []SpecProperty
	for _, sp := range specProps {
		if isExcludedProperty(sp.Name, excluded) || mapped[sp.Name] {
			continue
		}
		if sp.GovernsPath != "" && mappedPaths[sp.GovernsPath] {
			continue
		}
		if sp.GovernsPath != "" && discoverTypeToTFType(sp.Type, sp.ItemType, sp.AdditionalPropertiesType) != "" {
			unmapped = append(unmapped, sp)
		}
	}

	// Sort for deterministic output
	sortSpecProperties(unmapped)

	// Output YAML entries
	fmt.Println("# --- Auto-discovered entries from OpenAPI spec ---")
	fmt.Println("# Add these to internal/codegen/mappings.yaml")
	fmt.Println()

	goFields := make([]string, 0, len(unmapped))

	for _, sp := range unmapped {
		tfName := deriveTerraformName(sp.Name)
		goField := toGoFieldName(tfName)
		tfType := discoverTypeToTFType(sp.Type, sp.ItemType, sp.AdditionalPropertiesType)
		desc := cleanDescription(sp.Description)

		fmt.Printf("  - name: %s\n", tfName)
		fmt.Printf("    go_field: %s\n", goField)
		fmt.Printf("    type: %s\n", tfType)
		fmt.Printf("    openapi_property: %s\n", sp.Name)
		fmt.Printf("    description: %q\n", desc)
		fmt.Println()

		// Collect Go struct field lines
		goType := "types.String"
		switch tfType {
		case typeBool:
			goType = "types.Bool"
		case typeInt64:
			goType = "types.Int64"
		case typeListString:
			goType = "types.List"
		case typeMapString:
			goType = "types.Map"
		}
		goFields = append(goFields, fmt.Sprintf("\t%s %s `tfsdk:\"%s\"`", goField, goType, tfName))
	}

	// Output Go struct fields
	fmt.Println("// --- Go struct fields to add to ProjectConfigResourceModel ---")
	fmt.Println("// Add these to internal/resources/projectconfig/resource.go")
	fmt.Println()
	for _, f := range goFields {
		fmt.Println(f)
	}
	fmt.Printf("\n// Total: %d new attributes\n", len(unmapped))
}

// collectUnmappedProperties returns the spec properties that have a governs path
// and a supported type but are not yet present in mappings. Shared by
// discoverNewEntries and applyDiscoveredEntries so both produce the same set.
func collectUnmappedProperties(m Mappings, specProps map[string]SpecProperty) []SpecProperty {
	mapped := make(map[string]bool)
	mappedPaths := make(map[string]bool)
	for _, a := range m.Attributes {
		if a.OpenAPIProperty != "" {
			mapped[a.OpenAPIProperty] = true
		}
		if a.PatchPath != "" {
			mappedPaths[a.PatchPath] = true
		}
	}
	excluded := excludedProperties()

	var unmapped []SpecProperty
	for _, sp := range specProps {
		if isExcludedProperty(sp.Name, excluded) || mapped[sp.Name] {
			continue
		}
		if sp.GovernsPath != "" && mappedPaths[sp.GovernsPath] {
			continue
		}
		if sp.GovernsPath != "" && discoverTypeToTFType(sp.Type, sp.ItemType, sp.AdditionalPropertiesType) != "" {
			unmapped = append(unmapped, sp)
		}
	}
	sortSpecProperties(unmapped)
	return unmapped
}

// applyDiscoveredEntries appends YAML entries for unmapped properties to
// mappingsPath and inserts matching Go struct fields into resourcePath's
// ProjectConfigResourceModel struct. Returns the number of entries added.
func applyDiscoveredEntries(m Mappings, specProps map[string]SpecProperty, mappingsPath, resourcePath string) (int, error) {
	unmapped := collectUnmappedProperties(m, specProps)
	if len(unmapped) == 0 {
		return 0, nil
	}

	var yamlBuf strings.Builder
	yamlBuf.WriteString("\n  # --- Auto-discovered entries (review names/descriptions before merging) ---\n")
	goFields := make([]string, 0, len(unmapped))
	for _, sp := range unmapped {
		tfName := deriveTerraformName(sp.Name)
		goField := toGoFieldName(tfName)
		tfType := discoverTypeToTFType(sp.Type, sp.ItemType, sp.AdditionalPropertiesType)
		desc := cleanDescription(sp.Description)

		fmt.Fprintf(&yamlBuf, "  - name: %s\n", tfName)
		fmt.Fprintf(&yamlBuf, "    go_field: %s\n", goField)
		fmt.Fprintf(&yamlBuf, "    type: %s\n", tfType)
		fmt.Fprintf(&yamlBuf, "    openapi_property: %s\n", sp.Name)
		fmt.Fprintf(&yamlBuf, "    description: %q\n", desc)
		yamlBuf.WriteString("\n")

		goType := "types.String"
		switch tfType {
		case typeBool:
			goType = "types.Bool"
		case typeInt64:
			goType = "types.Int64"
		case typeListString:
			goType = "types.List"
		case typeMapString:
			goType = "types.Map"
		}
		goFields = append(goFields, fmt.Sprintf("\t%s %s `tfsdk:\"%s\"`", goField, goType, tfName))
	}

	if err := appendToMappingsFile(mappingsPath, yamlBuf.String()); err != nil {
		return 0, fmt.Errorf("appending to mappings: %w", err)
	}
	if err := insertStructFields(resourcePath, "ProjectConfigResourceModel", goFields); err != nil {
		return 0, fmt.Errorf("inserting struct fields: %w", err)
	}
	return len(unmapped), nil
}

// appendToMappingsFile appends text to the mappings file, ensuring the file
// ends with a single newline before the appended content.
func appendToMappingsFile(path, content string) error {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) //nolint:gosec // path from trusted CLI flag
	if err != nil {
		return err
	}
	// Ensure the existing file ends with exactly one newline before our block.
	trimmed := strings.TrimRight(string(data), "\n") + "\n"
	// #nosec G703 -- path from trusted CLI flag, not user input
	return os.WriteFile(cleanPath, []byte(trimmed+content), 0o600)
}

// insertStructFields inserts the given field lines immediately before the
// closing brace of the named top-level struct. The struct is expected to be
// declared at column 0 with `type <name> struct {` and closed with a `}` at
// column 0.
func insertStructFields(path, structName string, fields []string) error {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) //nolint:gosec // path from trusted CLI flag
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	startPrefix := fmt.Sprintf("type %s struct {", structName)
	startIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, startPrefix) {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return fmt.Errorf("struct %q not found in %s", structName, path)
	}
	closeIdx := -1
	for i := startIdx + 1; i < len(lines); i++ {
		if lines[i] == "}" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return fmt.Errorf("closing brace of struct %q not found in %s", structName, path)
	}

	insertion := append([]string{"", "\t// Auto-discovered (review naming before release)"}, fields...)
	newLines := make([]string, 0, len(lines)+len(insertion))
	newLines = append(newLines, lines[:closeIdx]...)
	newLines = append(newLines, insertion...)
	newLines = append(newLines, lines[closeIdx:]...)

	// #nosec G703 -- path from trusted CLI flag, not user input
	return os.WriteFile(cleanPath, []byte(strings.Join(newLines, "\n")), 0o600)
}

// sortSpecProperties sorts spec properties by name for deterministic output.
func sortSpecProperties(props []SpecProperty) {
	sort.Slice(props, func(i, j int) bool {
		return props[i].Name < props[j].Name
	})
}

// deriveTerraformName converts an OpenAPI property name to a terraform attribute name.
// kratos_selfservice_flows_login_style -> selfservice_flows_login_style
// hydra_oauth2_pkce_enforced -> oauth2_pkce_enforced
func deriveTerraformName(openapiName string) string {
	// Strip known prefixes
	for _, prefix := range []string{"kratos_", "hydra_"} {
		if strings.HasPrefix(openapiName, prefix) {
			stripped := strings.TrimPrefix(openapiName, prefix)
			if prefix == "hydra_" && !strings.HasPrefix(stripped, "oauth2_") && !strings.HasPrefix(stripped, "oidc_") {
				// Add oauth2_ prefix for hydra settings that don't already have it
				return "oauth2_" + stripped
			}
			return stripped
		}
	}
	// Keep keto_ and account_experience_ prefixes as-is
	return openapiName
}

// toGoFieldName converts a terraform attribute name to a Go struct field name.
// selfservice_flows_login_style -> SelfserviceFlowsLoginStyle
// oauth2_pkce_enforced -> OAuth2PKCEEnforced
func toGoFieldName(tfName string) string {
	parts := strings.Split(tfName, "_")
	var result strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if upper, ok := acronymMap[strings.ToLower(part)]; ok {
			result.WriteString(upper)
		} else {
			result.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return result.String()
}

var acronymMap = map[string]string{
	"oauth2":   "OAuth2",
	"oidc":     "OIDC",
	"pkce":     "PKCE",
	"jwt":      "JWT",
	"url":      "URL",
	"uri":      "URI",
	"ui":       "UI",
	"smtp":     "SMTP",
	"mfa":      "MFA",
	"totp":     "TOTP",
	"aal":      "AAL",
	"rp":       "RP",
	"http":     "HTTP",
	"id":       "ID",
	"webauthn": "WebAuthn",
	"html":     "HTML",
	"sms":      "SMS",
	"ttl":      "TTL",
	"ip":       "IP",
	"css":      "CSS",
	"api":      "API",
	"saml":     "SAML",
	"jti":      "JTI",
	"iat":      "IAT",
	"b2b":      "B2B",
	"sso":      "SSO",
}

// excludedProperties returns properties that should be excluded from unmapped
// reports because they are read-only, managed by other resources, or handled
// by custom code in resource.go under a different terraform attribute name.
func excludedProperties() map[string]bool {
	return map[string]bool{
		// Read-only / metadata
		"id": true, "project_id": true, "created_at": true, "updated_at": true,
		"name": true, "state": true, "workspace_id": true, "production": true,
		"strict_security": true, "project_name": true,
		// Managed by separate terraform resources
		"kratos_identity_schemas":                          true,
		"kratos_selfservice_methods_oidc_config_providers": true,
		"kratos_selfservice_methods_saml_config_providers": true,
		"organizations":                          true,
		"project_revision_hooks":                 true,
		"scim_clients":                           true,
		"account_experience_custom_translations": true,
		// Handled by custom code in resource.go under different terraform names
		"serve_public_cors_enabled":                             true, // → cors_enabled
		"serve_public_cors_allowed_origins":                     true, // → cors_origins
		"serve_admin_cors_enabled":                              true, // → cors_admin_enabled
		"serve_admin_cors_allowed_origins":                      true, // → cors_admin_origins
		"keto_namespaces":                                       true, // → keto_namespaces (custom list handler)
		"kratos_courier_channels":                               true, // → courier_channels (custom nested)
		"kratos_courier_smtp_headers":                           true, // → smtp_headers (custom map)
		"kratos_courier_http_request_config_headers":            true, // → part of courier_http_request_config
		"kratos_courier_http_request_config_auth_type":          true, // → part of courier_http_request_config
		"kratos_courier_http_request_config_method":             true, // → part of courier_http_request_config
		"kratos_courier_delivery_strategy":                      true, // → courier_delivery_strategy (in mappings)
		"kratos_selfservice_allowed_return_urls":                true, // → allowed_return_urls (custom)
		"kratos_selfservice_methods_webauthn_config_rp_origins": true, // → webauthn_rp_origins (custom)
		"hydra_oauth2_allowed_top_level_claims":                 true, // → oauth2_allowed_top_level_claims (custom)
		"kratos_session_whoami_tokenizer_templates":             true, // → session_tokenizer_templates (custom)
		"kratos_courier_smtp_connection_uri":                    true, // → smtp_connection_uri (custom, sensitive)
		"account_experience_default_locale":                     true, // → account_experience_default_locale (in mappings)
		"kratos_oauth2_provider_headers":                        true, // → oauth2_provider_headers (in mappings)

		// Account experience images — handled by custom code in resource.go.
		// The spec's governs descriptions point at favicon_*/logo_* config keys,
		// but the API stores them at favicon_*_url/logo_*_url, expects an inline
		// data URI on write, and returns a content-addressed storage URL on read
		// (filename is sha512 of the image bytes). See issue #250.
		"account_experience_favicon_dark":  true, // → account_experience_favicon_dark (custom)
		"account_experience_favicon_light": true, // → account_experience_favicon_light (custom)
		"account_experience_logo_dark":     true, // → account_experience_logo_dark (custom)
		"account_experience_logo_light":    true, // → account_experience_logo_light (custom)

		// Courier templates (kratos_courier_templates_*) are excluded by prefix in
		// isExcludedProperty; see courierTemplatePropertyPrefix. Enumerating each
		// family here previously missed new ones (e.g. verifiable_address_changed),
		// which then leaked into the generated schema. See issue #213.
	}
}

// isExcludedProperty reports whether a spec property should be skipped during
// discovery and coverage checks: either it is in the static exclusion set or it
// is a courier template property (excluded by prefix; see
// courierTemplatePropertyPrefix).
func isExcludedProperty(name string, excluded map[string]bool) bool {
	return excluded[name] || strings.HasPrefix(name, courierTemplatePropertyPrefix)
}

// endsWithSentencePunctuation reports whether s ends with terminal punctuation,
// so cleanDescription does not insert a redundant period when joining paragraphs.
func endsWithSentencePunctuation(s string) bool {
	if s == "" {
		return false
	}
	switch s[len(s)-1] {
	case '.', '!', '?', ':', ';':
		return true
	}
	return false
}

// cleanDescription removes the "governs" sentence, joins multi-paragraph spec
// docs into space-separated sentences, strips "Ory Kratos" / "Ory Hydra"
// prefixes, and truncates for Terraform docs.
func cleanDescription(desc string) string {
	cleaned := governsRegex.ReplaceAllString(desc, "")

	// Join blank-line-separated paragraphs into space-separated sentences. Each
	// paragraph is whitespace-normalized; a paragraph that does not already end in
	// sentence punctuation gets a period so neighboring paragraphs do not run
	// together (e.g. "...for testing" + "Only allowed..." -> "...for testing. Only
	// allowed...").
	paragraphs := paragraphBreakRegex.Split(cleaned, -1)
	sentences := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		if p = strings.Join(strings.Fields(p), " "); p != "" {
			sentences = append(sentences, p)
		}
	}
	for i := 0; i < len(sentences)-1; i++ {
		if !endsWithSentencePunctuation(sentences[i]) {
			sentences[i] += "."
		}
	}
	cleaned = strings.Join(sentences, " ")

	// Strip "Configures the " / "Configures whether " prefixes for brevity.
	// Must happen before product-name stripping so "Configures the Ory Hydra ..."
	// becomes "Ory Hydra ..." and then the product prefix can be removed.
	cleaned = strings.TrimPrefix(cleaned, "Configures the ")
	cleaned = strings.TrimPrefix(cleaned, "Configures whether ")
	cleaned = strings.TrimPrefix(cleaned, "Configures ")

	// Strip "Ory Kratos" / "Ory Hydra" / "Ory Keto" at the start to avoid
	// vendor prefixes in generated docs.
	cleaned = strings.TrimPrefix(cleaned, "Ory Kratos ")
	cleaned = strings.TrimPrefix(cleaned, "Ory Hydra ")
	cleaned = strings.TrimPrefix(cleaned, "Ory Keto ")

	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.TrimSuffix(cleaned, ".")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return "Configuration setting."
	}

	// Capitalize first letter
	if len(cleaned) > 0 && cleaned[0] >= 'a' && cleaned[0] <= 'z' {
		cleaned = strings.ToUpper(cleaned[:1]) + cleaned[1:]
	}

	// Truncate to first sentence if very long (>200 chars)
	if len(cleaned) > 200 {
		if idx := strings.Index(cleaned, ". "); idx > 0 && idx < 200 {
			cleaned = cleaned[:idx]
		} else if len(cleaned) > 200 {
			cleaned = cleaned[:197] + "..."
		}
	}

	return cleaned
}

func buildSchemaAttr(a Attribute) string {
	var b strings.Builder

	switch a.Type {
	case typeString:
		b.WriteString("schema.StringAttribute{\n")
	case typeBool:
		b.WriteString("schema.BoolAttribute{\n")
	case typeInt64:
		b.WriteString("schema.Int64Attribute{\n")
	case typeListString:
		b.WriteString("schema.ListAttribute{\n")
	case typeMapString:
		b.WriteString("schema.MapAttribute{\n")
	}

	fmt.Fprintf(&b, "\t\t\tDescription: %q,\n", a.Description)
	b.WriteString("\t\t\tOptional:    true,\n")

	if a.Type == typeListString || a.Type == typeMapString {
		b.WriteString("\t\t\tElementType: types.StringType,\n")
	}

	// Skip Computed and Defaults on primary attributes that have deprecated aliases,
	// otherwise Terraform applies the default to the primary even when only the
	// deprecated alias is set, breaking the fallback logic.
	hasAlias := a.DeprecatedName != ""
	if a.Computed && !hasAlias {
		b.WriteString("\t\t\tComputed:    true,\n")
	}
	if a.Sensitive {
		b.WriteString("\t\t\tSensitive:   true,\n")
	}
	if a.DefaultBool != nil && !hasAlias {
		fmt.Fprintf(&b, "\t\t\tDefault:     booldefault.StaticBool(%v),\n", *a.DefaultBool)
	}
	if a.DefaultInt64 != nil && !hasAlias {
		fmt.Fprintf(&b, "\t\t\tDefault:     int64default.StaticInt64(%d),\n", *a.DefaultInt64)
	}

	if a.Validators != nil && a.Type == typeString {
		var validatorExprs []string
		if len(a.Validators.OneOf) > 0 {
			quoted := make([]string, len(a.Validators.OneOf))
			for i, s := range a.Validators.OneOf {
				quoted[i] = fmt.Sprintf("%q", s)
			}
			validatorExprs = append(validatorExprs, fmt.Sprintf("stringvalidator.OneOf(%s)", strings.Join(quoted, ", ")))
		}
		if a.Validators.Regex != "" {
			msg := a.Validators.RegexMessage
			if msg == "" {
				msg = "must match pattern"
			}
			validatorExprs = append(validatorExprs, fmt.Sprintf("stringvalidator.RegexMatches(\n\t\t\t\t\tregexp.MustCompile(%q),\n\t\t\t\t\t%q,\n\t\t\t\t)", a.Validators.Regex, msg))
		}
		if len(validatorExprs) > 0 {
			fmt.Fprintf(&b, "\t\t\tValidators: []validator.String{\n")
			for _, expr := range validatorExprs {
				fmt.Fprintf(&b, "\t\t\t\t%s,\n", expr)
			}
			fmt.Fprintf(&b, "\t\t\t},\n")
		}
	}

	b.WriteString("\t\t}")
	return b.String()
}

// buildDeprecatedSchemaAttr generates a schema attribute for a deprecated alias.
// It has the same type and description but includes a DeprecationMessage and
// omits validators, defaults, and computed; sensitive is preserved to avoid leaking secrets.
func buildDeprecatedSchemaAttr(a Attribute) string {
	var b strings.Builder

	switch a.Type {
	case typeString:
		b.WriteString("schema.StringAttribute{\n")
	case typeBool:
		b.WriteString("schema.BoolAttribute{\n")
	case typeInt64:
		b.WriteString("schema.Int64Attribute{\n")
	case typeListString:
		b.WriteString("schema.ListAttribute{\n")
	case typeMapString:
		b.WriteString("schema.MapAttribute{\n")
	}

	fmt.Fprintf(&b, "\t\t\tDescription: %q,\n", a.Description)
	b.WriteString("\t\t\tOptional:    true,\n")

	if a.Type == typeListString || a.Type == typeMapString {
		b.WriteString("\t\t\tElementType: types.StringType,\n")
	}

	if a.Sensitive {
		b.WriteString("\t\t\tSensitive:   true,\n")
	}

	fmt.Fprintf(&b, "\t\t\tDeprecationMessage: %q,\n",
		fmt.Sprintf("Use %s instead. This attribute will be removed in a future major version.", a.Name))

	b.WriteString("\t\t}")
	return b.String()
}

var schemaTemplate = `// Code generated by go generate; DO NOT EDIT.
// Source: internal/codegen/cmd/generate/main.go
// Mappings: internal/codegen/mappings.yaml

package projectconfig

import (
{{- if .HasRegex }}
	"regexp"
{{- end }}

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure imported packages are used.
var (
	_ = stringvalidator.OneOf
	_ = booldefault.StaticBool
	_ = int64default.StaticInt64
	_ validator.String
	_ = types.StringType
)

// simpleSchemaAttributes returns the schema attributes for all simple
// (string, bool, int64, list_string, map_string) configuration fields.
// Complex nested types (tokenizer templates, courier channels, etc.) are
// defined separately.
func simpleSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
{{- range .Attributes }}
		{{ printf "%q" .Name }}: {{ schemaAttr . }},
{{- if .DeprecatedName }}
		{{ printf "%q" .DeprecatedName }}: {{ deprecatedSchemaAttr . }},
{{- end }}
{{- end }}
	}
}
`

var patchesTemplate = `// Code generated by go generate; DO NOT EDIT.
// Source: internal/codegen/cmd/generate/main.go
// Mappings: internal/codegen/mappings.yaml

package projectconfig

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StringPatchEntry maps a string field to its JSON Patch path.
type StringPatchEntry struct {
	Field      *types.String
	Deprecated *types.String // fallback when Field is null (deprecated alias)
	Path       string
}

// BoolEnumValues holds the string values a config key stores in place of a
// JSON bool. Sending a raw bool to such a key is accepted with HTTP 200 but
// normalized by string comparison, so it silently stores the false variant.
type BoolEnumValues struct {
	True  string
	False string
}

// BoolPatchEntry maps a bool field to its JSON Patch path.
type BoolPatchEntry struct {
	Field      *types.Bool
	Deprecated *types.Bool // fallback when Field is null (deprecated alias)
	Path       string
	Enum       *BoolEnumValues // when set, patch the string enum instead of a raw bool
}

// Int64PatchEntry maps an int64 field to its JSON Patch path.
type Int64PatchEntry struct {
	Field      *types.Int64
	Deprecated *types.Int64 // fallback when Field is null (deprecated alias)
	Path       string
}

// ListStringPatchEntry maps a list(string) field to its JSON Patch path.
type ListStringPatchEntry struct {
	Field      *types.List
	Deprecated *types.List // fallback when Field is null (deprecated alias)
	Path       string
}

// MapStringPatchEntry maps a map(string) field to its JSON Patch path.
type MapStringPatchEntry struct {
	Field      *types.Map
	Deprecated *types.Map // fallback when Field is null (deprecated alias)
	Path       string
}

// simpleStringPatchEntries returns all simple string attribute patch mappings.
func simpleStringPatchEntries(plan *ProjectConfigResourceModel) []StringPatchEntry {
	return []StringPatchEntry{
{{- range filterType .Attributes "string" }}
		{&plan.{{ .GoField }}, {{ if .DeprecatedGoField }}&plan.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, {{ printf "%q" .PatchPath }}},
{{- end }}
	}
}

// simpleBoolPatchEntries returns all simple bool attribute patch mappings.
func simpleBoolPatchEntries(plan *ProjectConfigResourceModel) []BoolPatchEntry {
	return []BoolPatchEntry{
{{- range filterType .Attributes "bool" }}
{{- if not .RevisionProperty }}
		{&plan.{{ .GoField }}, {{ if .DeprecatedGoField }}&plan.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, {{ printf "%q" .PatchPath }}, {{ if .BoolEnum }}&BoolEnumValues{ {{ printf "%q" .BoolEnum.TrueValue }}, {{ printf "%q" .BoolEnum.FalseValue }} }{{ else }}nil{{ end }}},
{{- end }}
{{- end }}
	}
}

// revisionBoolPatchEntries returns bool attributes that are top-level
// normalized revision columns. They have no key in any service config
// document — a document patch is accepted with HTTP 200 and silently
// discarded — so they are applied via the normalized revision endpoint
// (client.PatchProjectRevision), with the op path being the normalized
// property name. Reads go through revisionBoolReadEntries against the
// normalized revision, not the project document.
func revisionBoolPatchEntries(plan *ProjectConfigResourceModel) []BoolPatchEntry {
	return []BoolPatchEntry{
{{- range filterType .Attributes "bool" }}
{{- if .RevisionProperty }}
		{&plan.{{ .GoField }}, nil, {{ printf "%q" (printf "/%s" .OpenAPIProperty) }}, nil},
{{- end }}
{{- end }}
	}
}

// simpleInt64PatchEntries returns all simple int64 attribute patch mappings.
func simpleInt64PatchEntries(plan *ProjectConfigResourceModel) []Int64PatchEntry {
	return []Int64PatchEntry{
{{- range filterType .Attributes "int64" }}
		{&plan.{{ .GoField }}, {{ if .DeprecatedGoField }}&plan.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, {{ printf "%q" .PatchPath }}},
{{- end }}
	}
}

// simpleListStringPatchEntries returns all simple list(string) attribute patch mappings.
func simpleListStringPatchEntries(plan *ProjectConfigResourceModel) []ListStringPatchEntry {
	return []ListStringPatchEntry{
{{- range filterType .Attributes "list_string" }}
		{&plan.{{ .GoField }}, {{ if .DeprecatedGoField }}&plan.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, {{ printf "%q" .PatchPath }}},
{{- end }}
	}
}

// simpleMapStringPatchEntries returns all simple map(string) attribute patch mappings.
func simpleMapStringPatchEntries(plan *ProjectConfigResourceModel) []MapStringPatchEntry {
	return []MapStringPatchEntry{
{{- range filterType .Attributes "map_string" }}
		{&plan.{{ .GoField }}, {{ if .DeprecatedGoField }}&plan.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, {{ printf "%q" .PatchPath }}},
{{- end }}
	}
}
`

var readTemplate = `// Code generated by go generate; DO NOT EDIT.
// Source: internal/codegen/cmd/generate/main.go
// Mappings: internal/codegen/mappings.yaml

package projectconfig

import (
	"context"
	"math"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
)

// Ensure imports are used.
var (
	_ *ory.Project
	_ context.Context
	_ attr.Value
)

// StringReadEntry maps a string state field to its config read path.
type StringReadEntry struct {
	Field      *types.String
	Deprecated *types.String // fallback: used only when the primary Field is null in state
	Keys       []string
	SkipEmpty  bool
	// PreserveOnMissing keeps the state value when the API omits Keys, instead
	// of nulling it to surface drift. Set for secrets the API never echoes back
	// and for attributes it accepts but never reports, where a null would show a
	// diff on every plan that no apply can settle.
	PreserveOnMissing bool
	// StorageURL marks a field whose content the API uploads to object storage
	// and reports back as an https URL. Read resolves that URL to the value the
	// configuration holds. See resolveStorageURLContent in helpers.go.
	StorageURL bool
}

// BoolReadEntry maps a bool state field to its config read path.
type BoolReadEntry struct {
	Field             *types.Bool
	Deprecated        *types.Bool // fallback: used only when the primary Field is null in state
	Keys              []string
	PreserveOnMissing bool
	Enum              *BoolEnumValues // when set, the key stores a string enum; map it back by comparing to Enum.True
}

// Int64ReadEntry maps an int64 state field to its config read path.
type Int64ReadEntry struct {
	Field             *types.Int64
	Deprecated        *types.Int64 // fallback: used only when the primary Field is null in state
	Keys              []string
	PreserveOnMissing bool
}

// ListStringReadEntry maps a list(string) state field to its config read path.
type ListStringReadEntry struct {
	Field             *types.List
	Deprecated        *types.List // fallback: used only when the primary Field is null in state
	Keys              []string
	PreserveOnMissing bool
}

// MapStringReadEntry maps a map(string) state field to its config read path.
type MapStringReadEntry struct {
	Field             *types.Map
	Deprecated        *types.Map // fallback: used only when the primary Field is null in state
	Keys              []string
	PreserveOnMissing bool
}

// revisionBoolReadEntries returns bool attributes read from the normalized
// project revision (client.GetProjectNormalizedRevision) instead of the
// project document, keyed by their normalized property name. See
// readRevisionProperties in resource.go.
func revisionBoolReadEntries(state *ProjectConfigResourceModel) []BoolReadEntry {
	return []BoolReadEntry{
{{- range filterType .Attributes "bool" }}
{{- if .RevisionProperty }}
		{&state.{{ .GoField }}, nil, []string{ {{ printf "%q" .OpenAPIProperty }} }, false, nil},
{{- end }}
{{- end }}
	}
}

// readSimpleFields reads all simple attributes from the API response into state.
//
// Missing-key semantics: the API prunes empty values (empty string, empty
// list, empty map, integer zero) from stored configs, so a key the response
// omits is indistinguishable from an applied empty value. When state already
// holds the empty value for the type, the omission is exactly what the write
// produced and the value is kept. Any other state value is nulled so genuine
// out-of-band removals still surface as drift.
func readSimpleFields(ctx context.Context, project *ory.Project, state *ProjectConfigResourceModel) {
{{- range $svc := .ServiceNames }}
{{- $attrs := index $.ByService $svc }}
{{- $svcCfg := index $.Services $svc }}
	if {{ $svcCfg.NilCheck }} {
		{{ varName $svc }}Config := {{ $svcCfg.ConfigExpr }}
{{- $strings := filterType $attrs "string" }}
{{- if $strings }}
		for _, e := range {{ $svc }}StringReadEntries(state) {
			target := e.Field
			if target.IsNull() && e.Deprecated != nil && !e.Deprecated.IsNull() {
				target = e.Deprecated
			}
			if !target.IsNull() {
				if v, ok := getNestedString({{ varName $svc }}Config, e.Keys...); ok {
					if e.StorageURL {
						v = resolveStorageURLContent(ctx, v, target.ValueString())
					}
					if !e.SkipEmpty || v != "" {
						*target = types.StringValue(v)
					}
				} else if !e.PreserveOnMissing && target.ValueString() != "" {
					*target = types.StringNull()
				}
			}
		}
{{- end }}
{{- $bools := filterType $attrs "bool" }}
{{- if $bools }}
		for _, e := range {{ $svc }}BoolReadEntries(state) {
			target := e.Field
			if target.IsNull() && e.Deprecated != nil && !e.Deprecated.IsNull() {
				target = e.Deprecated
			}
			if !target.IsNull() {
				if e.Enum != nil {
					if v, ok := getNestedString({{ varName $svc }}Config, e.Keys...); ok {
						*target = types.BoolValue(v == e.Enum.True)
					} else if !e.PreserveOnMissing {
						*target = types.BoolNull()
					}
				} else if v, ok := getNestedBool({{ varName $svc }}Config, e.Keys...); ok {
					*target = types.BoolValue(v)
				} else if !e.PreserveOnMissing {
					*target = types.BoolNull()
				}
			}
		}
{{- end }}
{{- $ints := filterType $attrs "int64" }}
{{- if $ints }}
		for _, e := range {{ $svc }}Int64ReadEntries(state) {
			target := e.Field
			if target.IsNull() && e.Deprecated != nil && !e.Deprecated.IsNull() {
				target = e.Deprecated
			}
			if !target.IsNull() {
				if v, ok := getNestedFloat({{ varName $svc }}Config, e.Keys...); ok {
					if v == math.Trunc(v) {
						*target = types.Int64Value(int64(v))
					}
				} else if !e.PreserveOnMissing && target.ValueInt64() != 0 {
					*target = types.Int64Null()
				}
			}
		}
{{- end }}
{{- $listStrings := filterType $attrs "list_string" }}
{{- if $listStrings }}
		for _, e := range {{ $svc }}ListStringReadEntries(state) {
			target := e.Field
			if target.IsNull() && e.Deprecated != nil && !e.Deprecated.IsNull() {
				target = e.Deprecated
			}
			if !target.IsNull() {
				if v := getNestedValue({{ varName $svc }}Config, e.Keys...); v != nil {
					if arr, ok := v.([]interface{}); ok {
						strs := make([]string, 0, len(arr))
						allStrings := true
						for _, item := range arr {
							if s, ok := item.(string); ok {
								strs = append(strs, s)
							} else {
								allStrings = false
								break
							}
						}
						if allStrings {
							listVal, diags := types.ListValueFrom(ctx, types.StringType, strs)
							if !diags.HasError() {
								*target = listVal
							}
						}
					}
				} else if !e.PreserveOnMissing && len(target.Elements()) > 0 {
					*target = types.ListNull(types.StringType)
				}
			}
		}
{{- end }}
{{- $mapStrings := filterType $attrs "map_string" }}
{{- if $mapStrings }}
		for _, e := range {{ $svc }}MapStringReadEntries(state) {
			target := e.Field
			if target.IsNull() && e.Deprecated != nil && !e.Deprecated.IsNull() {
				target = e.Deprecated
			}
			if !target.IsNull() {
				if v := getNestedValue({{ varName $svc }}Config, e.Keys...); v != nil {
					if m, ok := v.(map[string]interface{}); ok {
						strMap := make(map[string]attr.Value, len(m))
						allStrings := true
						for k, val := range m {
							if s, ok := val.(string); ok {
								strMap[k] = types.StringValue(s)
							} else {
								allStrings = false
								break
							}
						}
						if allStrings {
							mapVal, diags := types.MapValue(types.StringType, strMap)
							if !diags.HasError() {
								*target = mapVal
							}
						}
					}
				} else if !e.PreserveOnMissing && len(target.Elements()) > 0 {
					*target = types.MapNull(types.StringType)
				}
			}
		}
{{- end }}
	}
{{- end }}
}

{{- range $svc := .ServiceNames }}
{{- $attrs := index $.ByService $svc }}
{{- $strings := filterType $attrs "string" }}
{{- if $strings }}

func {{ $svc }}StringReadEntries(state *ProjectConfigResourceModel) []StringReadEntry {
	return []StringReadEntry{
{{- range $strings }}
{{- if not .WriteOnly }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .ReadKeysPath }} }, {{ .SkipEmptyRead }}, {{ .ReadPreservesOnMissing }}, {{ .StorageURLContent }}},
{{- end }}
{{- end }}
	}
}
{{- end }}

{{- $bools := filterType $attrs "bool" }}
{{- if $bools }}

func {{ $svc }}BoolReadEntries(state *ProjectConfigResourceModel) []BoolReadEntry {
	return []BoolReadEntry{
{{- range $bools }}
{{- if not .WriteOnly }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .ReadKeysPath }} }, {{ .ReadPreservesOnMissing }}, {{ if .BoolEnum }}&BoolEnumValues{ {{ printf "%q" .BoolEnum.TrueValue }}, {{ printf "%q" .BoolEnum.FalseValue }} }{{ else }}nil{{ end }}},
{{- end }}
{{- end }}
	}
}
{{- end }}

{{- $ints := filterType $attrs "int64" }}
{{- if $ints }}

func {{ $svc }}Int64ReadEntries(state *ProjectConfigResourceModel) []Int64ReadEntry {
	return []Int64ReadEntry{
{{- range $ints }}
{{- if not .WriteOnly }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .ReadKeysPath }} }, {{ .ReadPreservesOnMissing }}},
{{- end }}
{{- end }}
	}
}
{{- end }}

{{- $listStrings := filterType $attrs "list_string" }}
{{- if $listStrings }}

func {{ $svc }}ListStringReadEntries(state *ProjectConfigResourceModel) []ListStringReadEntry {
	return []ListStringReadEntry{
{{- range $listStrings }}
{{- if not .WriteOnly }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .ReadKeysPath }} }, {{ .ReadPreservesOnMissing }}},
{{- end }}
{{- end }}
	}
}
{{- end }}

{{- $mapStrings := filterType $attrs "map_string" }}
{{- if $mapStrings }}

func {{ $svc }}MapStringReadEntries(state *ProjectConfigResourceModel) []MapStringReadEntry {
	return []MapStringReadEntry{
{{- range $mapStrings }}
{{- if not .WriteOnly }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .ReadKeysPath }} }, {{ .ReadPreservesOnMissing }}},
{{- end }}
{{- end }}
	}
}
{{- end }}
{{- end }}
`
