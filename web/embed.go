package web

import "embed"

//go:embed all:build doc
var BuildFS embed.FS
