package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSchemasFromProjectConfig(t *testing.T) {
	t.Run("nil identity service", func(t *testing.T) {
		project := &ory.Project{
			Services: ory.ProjectServices{},
		}
		schemas, err := extractSchemasFromProjectConfig(context.Background(), project)
		require.NoError(t, err)
		assert.Empty(t, schemas)
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
		require.NoError(t, err)
		require.Len(t, schemas, 1)
		assert.Equal(t, "test-schema-hash", schemas[0].GetId())
		require.NotNil(t, schemas[0].Schema, "expected schema content to be decoded")
		assert.Equal(t, "object", schemas[0].Schema["type"])
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
		require.NoError(t, err)
		require.Len(t, schemas, 1)
		assert.Equal(t, "preset://username", schemas[0].GetId())
		require.NotNil(t, schemas[0].Schema, "expected schema to be empty object")
		assert.Empty(t, schemas[0].Schema)
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
		require.Error(t, err, "expected error for invalid base64")
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
		require.Error(t, err, "expected error for invalid JSON")
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
		require.NoError(t, err)
		assert.Len(t, schemas, 2)
	})
}

func TestExtractSchemasFromProjectConfig_HTTPS(t *testing.T) {
	// Test that an HTTPS schema URL in project config results in the fetched
	// JSON being populated in the schema container.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"object","properties":{"email":{"type":"string"}}}`))
	}))
	defer srv.Close()

	origClient := schemaFetchClient
	origChecker := hostChecker
	schemaFetchClient = srv.Client()
	hostChecker = func(context.Context, string) (bool, error) { return false, nil }
	defer func() {
		schemaFetchClient = origClient
		hostChecker = origChecker
	}()

	project := &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{
				Config: map[string]interface{}{
					"identity": map[string]interface{}{
						"schemas": []interface{}{
							map[string]interface{}{
								"id":  "https-schema-id",
								"url": srv.URL + "/schema.json",
							},
						},
					},
				},
			},
		},
	}

	schemas, err := extractSchemasFromProjectConfig(context.Background(), project)
	require.NoError(t, err)
	require.Len(t, schemas, 1)
	assert.Equal(t, "https-schema-id", schemas[0].GetId())
	require.NotNil(t, schemas[0].Schema, "expected schema content from HTTPS fetch")
	assert.Equal(t, "object", schemas[0].Schema["type"])
	props, ok := schemas[0].Schema["properties"].(map[string]interface{})
	require.True(t, ok, "expected properties map in schema")
	assert.Contains(t, props, "email")
}

func TestHasProjectClient(t *testing.T) {
	t.Run("configured", func(t *testing.T) {
		c := &OryClient{config: OryClientConfig{ProjectSlug: "slug", ProjectAPIKey: "key"}}
		assert.True(t, c.HasProjectClient())
	})
	t.Run("missing slug", func(t *testing.T) {
		c := &OryClient{config: OryClientConfig{ProjectAPIKey: "key"}}
		assert.False(t, c.HasProjectClient())
	})
	t.Run("missing key", func(t *testing.T) {
		c := &OryClient{config: OryClientConfig{ProjectSlug: "slug"}}
		assert.False(t, c.HasProjectClient())
	})
}
