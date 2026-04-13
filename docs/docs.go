// Package docs exposes a minimal Swagger 2 compatibility document.
// The maintained machine-readable API reference for this repository is
// `docs/openapi.yaml`.
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
  "swagger": "2.0",
  "info": {
    "title": "{{.Title}}",
    "description": "{{escape .Description}}",
    "version": "{{.Version}}"
  },
  "host": "{{.Host}}",
  "basePath": "{{.BasePath}}",
  "schemes": {{ marshal .Schemes }},
  "paths": {},
  "externalDocs": {
    "description": "Canonical OpenAPI document",
    "url": "./openapi.yaml"
  }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "v1",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{"http"},
	Title:            "Velarix API",
	Description:      "Swagger 2 compatibility stub. The maintained machine-readable route definitions for this repository live in docs/openapi.yaml.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
