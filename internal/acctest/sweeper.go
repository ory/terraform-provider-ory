//go:build acceptance

package acctest

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	testResourcePrefix = "tf-acc-test"
)

func init() {
	resource.AddTestSweepers("ory_identity", &resource.Sweeper{
		Name: "ory_identity",
		F:    sweepIdentities,
	})

	resource.AddTestSweepers("ory_oauth2_client", &resource.Sweeper{
		Name: "ory_oauth2_client",
		F:    sweepOAuth2Clients,
	})

	resource.AddTestSweepers("ory_organization", &resource.Sweeper{
		Name:         "ory_organization",
		F:            sweepOrganizations,
		Dependencies: []string{"ory_identity"},
	})

	resource.AddTestSweepers("ory_project", &resource.Sweeper{
		Name:         "ory_project",
		F:            sweepProjects,
		Dependencies: []string{"ory_identity", "ory_oauth2_client", "ory_organization"},
	})
}

func sweepIdentities(region string) error {
	log.Println("[INFO] Sweeping identities...")

	if os.Getenv("ORY_PROJECT_API_KEY") == "" {
		log.Println("[WARN] ORY_PROJECT_API_KEY not set, skipping identity sweep")
		return nil
	}

	c, err := getOryClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("[INFO] Identity sweep: not implemented (requires list API)")
	_ = c
	_ = ctx

	return nil
}

func sweepOAuth2Clients(region string) error {
	log.Println("[INFO] Sweeping OAuth2 clients...")

	if os.Getenv("ORY_PROJECT_API_KEY") == "" {
		log.Println("[WARN] ORY_PROJECT_API_KEY not set, skipping OAuth2 client sweep")
		return nil
	}

	c, err := getOryClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clients, _, err := c.ProjectAPI().OAuth2API.ListOAuth2Clients(ctx).Execute()
	if err != nil {
		return fmt.Errorf("failed to list OAuth2 clients: %w", err)
	}

	var errorList []error
	for _, client := range clients {
		name := client.GetClientName()
		if !strings.HasPrefix(name, testResourcePrefix) && !strings.HasPrefix(name, "Test") {
			continue
		}

		log.Printf("[INFO] Deleting OAuth2 client: %s (%s)", name, client.GetClientId())
		if err := c.DeleteOAuth2Client(ctx, client.GetClientId()); err != nil {
			log.Printf("[WARN] Failed to delete OAuth2 client %s: %v", client.GetClientId(), err)
			errorList = append(errorList, err)
		}
	}

	return condenseErrors(errorList)
}

func sweepOrganizations(region string) error {
	log.Println("[INFO] Sweeping organizations...")

	if os.Getenv("ORY_WORKSPACE_API_KEY") == "" {
		log.Println("[WARN] ORY_WORKSPACE_API_KEY not set, skipping organization sweep")
		return nil
	}

	c, err := getOryClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		log.Println("[WARN] ORY_PROJECT_ID not set, skipping organization sweep")
		return nil
	}

	orgs, _, err := c.ConsoleAPI().ProjectAPI.ListOrganizations(ctx, projectID).Execute()
	if err != nil {
		if strings.Contains(err.Error(), "feature_not_available") {
			log.Println("[INFO] Organization feature not available, skipping sweep")
			return nil
		}
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	var errorList []error
	for _, org := range orgs.Organizations {
		label := org.GetLabel()
		if !strings.HasPrefix(label, testResourcePrefix) && !strings.HasPrefix(label, "Test") {
			continue
		}

		log.Printf("[INFO] Deleting organization: %s (%s)", label, org.GetId())
		if err := c.DeleteOrganization(ctx, projectID, org.GetId()); err != nil {
			log.Printf("[WARN] Failed to delete organization %s: %v", org.GetId(), err)
			errorList = append(errorList, err)
		}
	}

	return condenseErrors(errorList)
}

func sweepProjects(region string) error {
	log.Println("[INFO] Sweeping projects...")

	if os.Getenv("ORY_WORKSPACE_API_KEY") == "" {
		log.Println("[WARN] ORY_WORKSPACE_API_KEY not set, skipping project sweep")
		return nil
	}

	if os.Getenv("ORY_SWEEP_PROJECTS") != "true" {
		log.Println("[INFO] Project sweep disabled (set ORY_SWEEP_PROJECTS=true to enable)")
		return nil
	}

	c, err := getOryClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	workspaceID := os.Getenv("ORY_WORKSPACE_ID")
	if workspaceID == "" {
		log.Println("[WARN] ORY_WORKSPACE_ID not set, skipping project sweep")
		return nil
	}

	projects, _, err := c.ConsoleAPI().WorkspaceAPI.ListWorkspaceProjects(ctx, workspaceID).Execute()
	if err != nil {
		return fmt.Errorf("failed to list workspace projects: %w", err)
	}

	var errorList []error
	for _, project := range projects.Projects {
		name := project.GetName()
		if !strings.HasPrefix(name, testResourcePrefix) && !strings.HasPrefix(name, "tf-test") {
			continue
		}

		createdAt := project.GetCreatedAt()
		if time.Since(createdAt) < time.Hour {
			log.Printf("[INFO] Skipping recent project: %s (created %v ago)", name, time.Since(createdAt))
			continue
		}

		log.Printf("[INFO] Deleting project: %s (%s)", name, project.GetId())
		if err := c.DeleteProject(ctx, project.GetId()); err != nil {
			log.Printf("[WARN] Failed to delete project %s: %v", project.GetId(), err)
			errorList = append(errorList, err)
		}
	}

	return condenseErrors(errorList)
}

func condenseErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("multiple errors occurred:\n  - %s", strings.Join(msgs, "\n  - "))
}
