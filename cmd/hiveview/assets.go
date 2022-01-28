package main

import "embed"

// embeddedAssets contains the static web server content.
//go:embed assets
var embeddedAssets embed.FS
