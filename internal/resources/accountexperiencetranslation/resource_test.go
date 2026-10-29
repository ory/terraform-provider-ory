//go:build acceptance

package accountexperiencetranslation_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

const (
	enName = "ory_account_experience_translation.en"
	deName = "ory_account_experience_translation.de"
)

// checkStored asserts, for each locale, whether the API stores custom
// translations for it. Terraform state cannot show a remove that the API
// skipped, and Delete tolerates an "already removed" error by design, so the
// check reads the project back.
func checkStored(t *testing.T, want map[string]bool) resource.TestCheckFunc {
	t.Helper()
	return func(*terraform.State) error {
		return acctest.Eventually(func() error {
			projectID := acctest.GetTestProjectID(t)
			for locale, stored := range want {
				_, ok, err := acctest.ProjectConfigValue(t, projectID,
					"/services/account_experience/config/translations/"+locale)
				if err != nil {
					return err
				}
				if ok != stored {
					return fmt.Errorf("project %s: locale %s stored = %t, want %t", projectID, locale, ok, stored)
				}
			}
			return nil
		})
	}
}

func expectMessages(name string, want map[string]string) statecheck.StateCheck {
	checks := make(map[string]knownvalue.Check, len(want))
	for key, text := range want {
		checks[key] = knownvalue.StringExact(text)
	}
	return statecheck.ExpectKnownValue(name, tfjsonpath.New("messages"), knownvalue.MapExact(checks))
}

func TestAccAccountExperienceTranslationResource_basic(t *testing.T) {
	updatedEN := map[string]string{
		"login.title":        "Welcome back to Example",
		"recovery.title":     "Forgot your password?",
		"registration.title": "Create your Example account",
	}

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkStored(t, map[string]bool{"en": false, "de": false}),
		Steps: []resource.TestStep{
			// Create two locales in one apply. Both write into the same
			// translations map, so neither write may drop the other.
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				ConfigStateChecks: []statecheck.StateCheck{
					expectMessages(enName, map[string]string{
						"login.title":    "Sign in to Example",
						"recovery.title": "Forgot your password?",
					}),
					expectMessages(deName, map[string]string{"login.title": "Anmelden"}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(enName, "id", "en"),
					resource.TestCheckResourceAttrSet(enName, "project_id"),
					checkStored(t, map[string]bool{"en": true, "de": true}),
				),
			},
			// Import with <project-id>/<locale>. State has no messages yet, so
			// the read downloads the stored document and must get the same map.
			{
				ResourceName: enName,
				ImportState:  true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					return acctest.GetTestProjectID(t) + "/en", nil
				},
				ImportStateVerify: true,
			},
			// Import with the locale alone, which uses the provider's project.
			{
				ResourceName:      deName,
				ImportState:       true,
				ImportStateId:     "de",
				ImportStateVerify: true,
			},
			// Change one message and add another. Only en changes, in place.
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", nil),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(enName, plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction(deName, plancheck.ResourceActionNoop),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					expectMessages(enName, updatedEN),
					expectMessages(deName, map[string]string{"login.title": "Anmelden"}),
				},
			},
			// Destroy one locale. The other one stays stored.
			{
				Config: acctest.LoadTestConfig(t, "testdata/en_only.tf.tmpl", nil),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(deName, plancheck.ResourceActionDestroy),
						plancheck.ExpectResourceAction(enName, plancheck.ResourceActionNoop),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					expectMessages(enName, updatedEN),
				},
				Check: checkStored(t, map[string]bool{"en": true, "de": false}),
			},
		},
	})
}
