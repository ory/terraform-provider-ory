// Package provider exposes the Ory Network Terraform provider to other Go
// modules. In-process bridges, such as Crossplane upjet in no-fork mode and
// Pulumi bridged providers, use it to serve the provider without the
// terraform CLI. The Terraform binary uses the same constructor.
package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"

	internalprovider "github.com/ory/terraform-provider-ory/internal/provider"
)

// New returns a function that creates the Ory Network provider. The provider
// reports version in its metadata and in the User-Agent header of API calls.
func New(version string) func() provider.Provider {
	return internalprovider.New(version)
}
