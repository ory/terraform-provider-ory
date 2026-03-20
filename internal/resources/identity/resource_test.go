//go:build acceptance

package identity_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccIdentityResource_basic(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Username": "test-basic-user"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "schema_id", "preset://username"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_identity.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Password is write-only, not returned on read
				ImportStateVerifyIgnore: []string{"password"},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Username": "test-updated-user"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
				),
			},
		},
	})
}

func TestAccIdentityResource_withMetadata(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_metadata.tf.tmpl", map[string]string{"Username": "test-metadata-user"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
					resource.TestCheckResourceAttrSet("ory_identity.test", "metadata_public"),
				),
			},
		},
	})
}

func TestAccIdentityResource_withResourceCredentials(t *testing.T) {
	project := acctest.GetTestProject(t)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with resource-level project credentials
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_resource_credentials.tf.tmpl", map[string]string{
					"ProjectSlug":   project.Slug,
					"ProjectAPIKey": project.APIKey,
					"Username":      "test-creds-user",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "schema_id", "preset://username"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
					resource.TestCheckResourceAttr("ory_identity.test", "project_slug", project.Slug),
				),
			},
			// ImportState — ignore resource-level credentials (not in API response)
			{
				ResourceName:            "ory_identity.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "project_slug", "project_api_key"},
			},
		},
	})
}

func TestAccIdentityResource_inactive(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/inactive.tf.tmpl", map[string]string{"Username": "test-inactive-user"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "inactive"),
				),
			},
		},
	})
}
