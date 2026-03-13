package client

import (
	"context"
	"testing"

	ory "github.com/ory/client-go"
)

func TestExtractSchemasFromProjectConfig(t *testing.T) {
	t.Run("nil identity service", func(t *testing.T) {
		project := &ory.Project{
			Services: ory.ProjectServices{},
		}
		schemas, err := extractSchemasFromProjectConfig(context.Background(), project)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(schemas) != 0 {
			t.Fatalf("expected 0 schemas, got %d", len(schemas))
		}
	})

	t.Run("base64 schema", func(t *testing.T) {
		// {"type":"object","properties":{"traits":{"type":"object"}}}
		b64 := "eyJ0eXBlIjoib2JqZWN0IiwicHJvcGVydGllcyI6eyJ0cmFpdHMiOnsidHlwZSI6Im9iamVjdCJ9fX0="
		project := &ory.Project{
			Services: ory.ProjectServices{
				Identity: &ory.ProjectServiceIdentity{
					Config: map[string]interface{}{
						"identity": map[string]interface{}{
							"schemas": []interface{}{
								map[string]interface{}{
									"id":  "test-schema-hash",
									"url": "base64://" + b64,
								},
							},
						},
					},
				},
			},
		}
		schemas, err := extractSchemasFromProjectConfig(context.Background(), project)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(schemas) != 1 {
			t.Fatalf("expected 1 schema, got %d", len(schemas))
		}
		if schemas[0].GetId() != "test-schema-hash" {
			t.Errorf("expected id 'test-schema-hash', got %q", schemas[0].GetId())
		}
		if schemas[0].Schema == nil {
			t.Fatal("expected schema content to be decoded, got nil")
		}
		if schemas[0].Schema["type"] != "object" {
			t.Errorf("expected schema type 'object', got %v", schemas[0].Schema["type"])
		}
	})

	t.Run("preset schema returns empty object", func(t *testing.T) {
		project := &ory.Project{
			Services: ory.ProjectServices{
				Identity: &ory.ProjectServiceIdentity{
					Config: map[string]interface{}{
						"identity": map[string]interface{}{
							"schemas": []interface{}{
								map[string]interface{}{
									"id":  "preset://username",
									"url": "preset://username",
								},
							},
						},
					},
				},
			},
		}
		schemas, err := extractSchemasFromProjectConfig(context.Background(), project)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(schemas) != 1 {
			t.Fatalf("expected 1 schema, got %d", len(schemas))
		}
		if schemas[0].GetId() != "preset://username" {
			t.Errorf("expected id 'preset://username', got %q", schemas[0].GetId())
		}
		if schemas[0].Schema == nil {
			t.Fatal("expected schema to be empty object, got nil")
		}
		if len(schemas[0].Schema) != 0 {
			t.Errorf("expected empty schema object, got %v", schemas[0].Schema)
		}
	})

	t.Run("invalid base64 returns error", func(t *testing.T) {
		project := &ory.Project{
			Services: ory.ProjectServices{
				Identity: &ory.ProjectServiceIdentity{
					Config: map[string]interface{}{
						"identity": map[string]interface{}{
							"schemas": []interface{}{
								map[string]interface{}{
									"id":  "bad-b64",
									"url": "base64://!!!not-valid-base64!!!",
								},
							},
						},
					},
				},
			},
		}
		_, err := extractSchemasFromProjectConfig(context.Background(), project)
		if err == nil {
			t.Fatal("expected error for invalid base64, got nil")
		}
	})

	t.Run("invalid JSON in base64 returns error", func(t *testing.T) {
		// "not json" in base64
		project := &ory.Project{
			Services: ory.ProjectServices{
				Identity: &ory.ProjectServiceIdentity{
					Config: map[string]interface{}{
						"identity": map[string]interface{}{
							"schemas": []interface{}{
								map[string]interface{}{
									"id":  "bad-json",
									"url": "base64://bm90IGpzb24=",
								},
							},
						},
					},
				},
			},
		}
		_, err := extractSchemasFromProjectConfig(context.Background(), project)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})

	t.Run("multiple schemas", func(t *testing.T) {
		b64 := "eyJ0eXBlIjoib2JqZWN0In0=" // {"type":"object"}
		project := &ory.Project{
			Services: ory.ProjectServices{
				Identity: &ory.ProjectServiceIdentity{
					Config: map[string]interface{}{
						"identity": map[string]interface{}{
							"schemas": []interface{}{
								map[string]interface{}{
									"id":  "preset://username",
									"url": "preset://username",
								},
								map[string]interface{}{
									"id":  "custom-hash-id",
									"url": "base64://" + b64,
								},
							},
						},
					},
				},
			},
		}
		schemas, err := extractSchemasFromProjectConfig(context.Background(), project)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(schemas) != 2 {
			t.Fatalf("expected 2 schemas, got %d", len(schemas))
		}
	})
}

func TestHasProjectClient(t *testing.T) {
	t.Run("configured", func(t *testing.T) {
		c := &OryClient{config: OryClientConfig{ProjectSlug: "slug", ProjectAPIKey: "key"}}
		if !c.HasProjectClient() {
			t.Error("expected HasProjectClient to return true")
		}
	})
	t.Run("missing slug", func(t *testing.T) {
		c := &OryClient{config: OryClientConfig{ProjectAPIKey: "key"}}
		if c.HasProjectClient() {
			t.Error("expected HasProjectClient to return false")
		}
	})
	t.Run("missing key", func(t *testing.T) {
		c := &OryClient{config: OryClientConfig{ProjectSlug: "slug"}}
		if c.HasProjectClient() {
			t.Error("expected HasProjectClient to return false")
		}
	})
}
