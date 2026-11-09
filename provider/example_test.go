package provider_test

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/ory/terraform-provider-ory/provider"
)

// This example serves the provider in-process over the Terraform plugin
// protocol and reads its schema, as an in-process bridge does.
func ExampleNew() {
	server, err := providerserver.NewProtocol6WithError(provider.New("1.2.3")())()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		log.Fatal(err)
	}

	_, ok := resp.ResourceSchemas["ory_project_config"]
	fmt.Println(len(resp.Diagnostics), ok)
	// Output: 0 true
}
