//go:build acceptance

package acctest

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type CheckDestroyFunc func(ctx context.Context, id string) (bool, error)

func CheckDestroy(resourceType string, existsFn CheckDestroyFunc) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}

			id := rs.Primary.ID
			if id == "" {
				continue
			}

			exists, err := existsFn(ctx, id)
			if err != nil {
				if isNotFoundError(err) {
					continue
				}
				return fmt.Errorf("error checking if %s %s exists: %w", resourceType, id, err)
			}

			if exists {
				return fmt.Errorf("%s %s still exists after destroy", resourceType, id)
			}
		}

		return nil
	}
}

func CheckDestroyNoop() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return nil
	}
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "does not exist")
}

func IdentityExists(ctx context.Context, id string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	_, err = c.GetIdentity(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func OAuth2ClientExists(ctx context.Context, id string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	_, err = c.GetOAuth2Client(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func OrganizationExists(ctx context.Context, id string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	if sharedTestProject == nil {
		return false, fmt.Errorf("no test project available")
	}

	_, err = c.GetOrganization(ctx, sharedTestProject.ID, id)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func JWKSetExists(ctx context.Context, setID string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	_, err = c.GetJsonWebKeySet(ctx, setID)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func ProjectExists(ctx context.Context, id string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	_, err = c.GetProject(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func WorkspaceExists(ctx context.Context, id string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	_, err = c.GetWorkspace(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func RelationshipExists(ctx context.Context, id string) (bool, error) {
	c, err := getOryClient()
	if err != nil {
		return false, fmt.Errorf("failed to create client: %w", err)
	}

	parts := strings.Split(id, "/")
	if len(parts) != 4 {
		return false, fmt.Errorf("invalid relationship ID format: %s", id)
	}

	namespace := parts[0]
	object := parts[1]
	relation := parts[2]
	subjectID := parts[3]

	rels, err := c.GetRelationships(ctx, namespace, &object, &relation, &subjectID)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return len(rels.GetRelationTuples()) > 0, nil
}
