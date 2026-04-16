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
	Name            string     `yaml:"name"`
	GoField         string     `yaml:"go_field"`
	Type            string     `yaml:"type"` // string, bool, int64, list_string, map_string
	PatchPath       string     `yaml:"patch_path"`
	OpenAPIProperty string     `yaml:"openapi_property"` // maps to normalizedProjectRevision property name
	Description     string     `yaml:"description"`
	Computed        bool       `yaml:"computed"`
	DefaultBool     *bool      `yaml:"default_bool"`
	DefaultInt64    *int64     `yaml:"default_int64"`
	Sensitive       bool       `yaml:"sensitive"`
	SkipEmptyRead   bool       `yaml:"skip_empty_read"`
	Validators      *Validator `yaml:"validators"`

	// Deprecated alias support: when set, generates a second schema attribute
	// with the old name that shows a deprecation warning directing users to Name.
	DeprecatedName    string `yaml:"deprecated_name"`     // old terraform attribute name
	DeprecatedGoField string `yaml:"deprecated_go_field"` // old Go struct field name
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
	strict := flag.Bool("strict", false, "fail if any spec properties are unmapped (use in CI to detect drift)")
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

		// Report unmapped spec properties (candidates for new entries)
		unmappedCount := reportUnmapped(m, specProps)
		if *strict && unmappedCount > 0 {
			log.Fatalf("STRICT MODE: %d unmapped spec properties found. Run 'make discover' to generate YAML entries.", unmappedCount)
		}
	} else if *discover {
		log.Fatal("--discover requires --spec")
	} else if *strict {
		log.Fatal("--strict requires --spec")
	}

	// Validate all attributes have required fields
	for i, a := range m.Attributes {
		if a.Name == "" || a.GoField == "" || a.Type == "" || a.PatchPath == "" {
			log.Fatalf("attribute %d (%s): name, go_field, type, and patch_path are required (patch_path can be derived from spec via openapi_property)", i, a.Name)
		}
		if a.Type != typeString && a.Type != typeBool && a.Type != typeInt64 && a.Type != typeListString && a.Type != typeMapString {
			log.Fatalf("attribute %q: unsupported type %q", a.Name, a.Type)
		}
		if a.Description == "" {
			log.Fatalf("attribute %q: description is required (set it in mappings.yaml or ensure openapi_property points to a spec entry with a description)", a.Name)
		}
	}

	// Group by service
	byService := make(map[string][]Attribute)
	hasRegex := false
	for _, a := range m.Attributes {
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
		if excluded[sp.Name] {
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
		if excluded[sp.Name] || mapped[sp.Name] {
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
	}
}

// cleanDescription removes the "governs" sentence, collapses newlines,
// strips "Ory Kratos" / "Ory Hydra" prefixes, and truncates for Terraform docs.
func cleanDescription(desc string) string {
	cleaned := governsRegex.ReplaceAllString(desc, "")

	// Normalize whitespace in one pass
	cleaned = strings.Join(strings.Fields(cleaned), " ")

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

// BoolPatchEntry maps a bool field to its JSON Patch path.
type BoolPatchEntry struct {
	Field      *types.Bool
	Deprecated *types.Bool // fallback when Field is null (deprecated alias)
	Path       string
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
		{&plan.{{ .GoField }}, {{ if .DeprecatedGoField }}&plan.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, {{ printf "%q" .PatchPath }}},
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
}

// BoolReadEntry maps a bool state field to its config read path.
type BoolReadEntry struct {
	Field      *types.Bool
	Deprecated *types.Bool // fallback: used only when the primary Field is null in state
	Keys       []string
}

// Int64ReadEntry maps an int64 state field to its config read path.
type Int64ReadEntry struct {
	Field      *types.Int64
	Deprecated *types.Int64 // fallback: used only when the primary Field is null in state
	Keys       []string
}

// ListStringReadEntry maps a list(string) state field to its config read path.
type ListStringReadEntry struct {
	Field      *types.List
	Deprecated *types.List // fallback: used only when the primary Field is null in state
	Keys       []string
}

// MapStringReadEntry maps a map(string) state field to its config read path.
type MapStringReadEntry struct {
	Field      *types.Map
	Deprecated *types.Map // fallback: used only when the primary Field is null in state
	Keys       []string
}

// readSimpleFields reads all simple attributes from the API response into state.
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
					if !e.SkipEmpty || v != "" {
						*target = types.StringValue(v)
					}
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
				if v, ok := getNestedBool({{ varName $svc }}Config, e.Keys...); ok {
					*target = types.BoolValue(v)
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
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .PatchPath }} }, {{ .SkipEmptyRead }}},
{{- end }}
	}
}
{{- end }}

{{- $bools := filterType $attrs "bool" }}
{{- if $bools }}

func {{ $svc }}BoolReadEntries(state *ProjectConfigResourceModel) []BoolReadEntry {
	return []BoolReadEntry{
{{- range $bools }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .PatchPath }} }},
{{- end }}
	}
}
{{- end }}

{{- $ints := filterType $attrs "int64" }}
{{- if $ints }}

func {{ $svc }}Int64ReadEntries(state *ProjectConfigResourceModel) []Int64ReadEntry {
	return []Int64ReadEntry{
{{- range $ints }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .PatchPath }} }},
{{- end }}
	}
}
{{- end }}

{{- $listStrings := filterType $attrs "list_string" }}
{{- if $listStrings }}

func {{ $svc }}ListStringReadEntries(state *ProjectConfigResourceModel) []ListStringReadEntry {
	return []ListStringReadEntry{
{{- range $listStrings }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .PatchPath }} }},
{{- end }}
	}
}
{{- end }}

{{- $mapStrings := filterType $attrs "map_string" }}
{{- if $mapStrings }}

func {{ $svc }}MapStringReadEntries(state *ProjectConfigResourceModel) []MapStringReadEntry {
	return []MapStringReadEntry{
{{- range $mapStrings }}
		{&state.{{ .GoField }}, {{ if .DeprecatedGoField }}&state.{{ .DeprecatedGoField }}{{ else }}nil{{ end }}, []string{ {{ readKeys .PatchPath }} }},
{{- end }}
	}
}
{{- end }}
{{- end }}
`
