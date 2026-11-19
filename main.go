package main

import (
	"context"
	"flag"
	"log"

	"github.com/caltra-dev/terraform-provider-caltra/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Print connection information for provider debugging.")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/caltra-dev/caltra",
		Debug:   *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
