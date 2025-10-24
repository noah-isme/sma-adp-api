package swagger

import "github.com/swaggo/swag"

const docTemplate = `{
    "swagger": "2.0",
    "info": {
        "title": "SMA ADP API",
        "description": "Bootstrap server for Golang migration (Phase 0)",
        "version": "0.1.0"
    },
    "basePath": "/",
    "schemes": [
        "http"
    ],
    "paths": {
        "/health": {
            "get": {
                "summary": "Health check",
                "responses": {
                    "200": {
                        "description": "OK"
                    }
                }
            }
        },
        "/ready": {
            "get": {
                "summary": "Readiness check",
                "responses": {
                    "200": {
                        "description": "Ready"
                    }
                }
            }
        }
    }
}`

type swaggerDoc struct{}

// ReadDoc returns the Swagger document.
func (s *swaggerDoc) ReadDoc() string {
	return docTemplate
}

func init() {
	swag.Register(swag.Name, &swaggerDoc{})
}
