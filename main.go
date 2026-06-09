// Copyright (c) Vestmark
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/vestmark-infra/tf-provider-northflank/internal/provider"
)

// version is set by goreleaser at release time.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support (delve)")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// Must match the dev_overrides key and the registry address.
		Address: "registry.terraform.io/vestmark-infra/northflank",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
