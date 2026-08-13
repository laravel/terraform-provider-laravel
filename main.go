package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/laravel/terraform-provider-laravel/internal/provider"
)

// Regenerate docs/ from the provider schema plus the files under examples/.
// Pinned so CI and local runs produce byte-identical output. Run via
// `make generate`; the docs-drift job in CI fails if the result differs from
// what is committed.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.23.0 generate --provider-name laravel --rendered-provider-name "Laravel Cloud"

var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/laravel/laravel",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
