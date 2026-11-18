//go:build acceptance

package member_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/client"
)

func TestAccProjectMemberResource_basic(t *testing.T) {
	acctest.RequireMemberTests(t)
	testMemberLifecycle(t, "ory_project_member", client.ProjectScope, "testdata/project_member.tf.tmpl",
		func() string { return acctest.GetTestProjectID(t) })
}

func TestAccWorkspaceMemberResource_basic(t *testing.T) {
	acctest.RequireMemberTests(t)
	testMemberLifecycle(t, "ory_workspace_member", client.WorkspaceScope, "testdata/workspace_member.tf.tmpl",
		func() string { return os.Getenv("ORY_WORKSPACE_ID") })
}

// testMemberLifecycle invites an example.com address, imports the invite,
// changes the role, and checks that destroy cancels the invite. Nobody can
// accept the invite, so it stays pending.
func testMemberLifecycle(t *testing.T, resourceType string, scope client.MemberScope, config string, scopeID func() string) {
	t.Helper()
	address := resourceType + ".test"
	email := fmt.Sprintf("tf-acc-member-%d@example.com", time.Now().UnixNano())
	var firstInvite string

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkNoOpenInvite(scope, scopeID, email),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, config, map[string]string{"Email": email, "Role": "viewer"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "email", email),
					resource.TestCheckResourceAttr(address, "role", "viewer"),
					resource.TestCheckResourceAttr(address, "status", "pending"),
					resource.TestCheckNoResourceAttr(address, "member_id"),
					resource.TestCheckResourceAttrWith(address, "id", func(id string) error {
						if want := scopeID() + "/" + email; id != want {
							return fmt.Errorf("id is %q, want %q", id, want)
						}
						return nil
					}),
					resource.TestCheckResourceAttrWith(address, "invite_id", func(id string) error {
						firstInvite = id
						return nil
					}),
				),
			},
			{
				ResourceName:      address,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: acctest.LoadTestConfig(t, config, map[string]string{"Email": email, "Role": "developer"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "role", "developer"),
					resource.TestCheckResourceAttr(address, "status", "pending"),
					resource.TestCheckResourceAttrWith(address, "invite_id", func(id string) error {
						if id == firstInvite {
							return fmt.Errorf("a role change kept invite %s, want a new invite", id)
						}
						return nil
					}),
				),
			},
		},
	})
}

// checkNoOpenInvite fails while email still has a pending or accepted invite.
func checkNoOpenInvite(scope client.MemberScope, scopeID func() string, email string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		c, err := acctest.GetOryClient()
		if err != nil {
			return err
		}
		invites, err := c.ListMemberInvites(context.Background(), scope, scopeID())
		if err != nil {
			return err
		}
		for _, i := range invites {
			if i.InviteeEmail == email && (i.Status == client.InviteStatusPending || i.Status == client.InviteStatusAccepted) {
				return fmt.Errorf("invite %s for %s is still %s", i.ID, email, i.Status)
			}
		}
		return nil
	}
}
