// main.go
package main

import (
	"context"
	"log"

	"github.com/daudcanugerah/terraform-provider-kclx/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	if err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/daudcanugerah/kclx",
	}); err != nil {
		log.Fatal(err.Error())
	}
}
