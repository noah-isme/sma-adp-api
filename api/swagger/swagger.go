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
    "tags": [
        {"name": "Teachers", "description": "Teacher roster management"},
        {"name": "Teacher Assignments", "description": "Teacher ↔ class/subject roster"},
        {"name": "Teacher Preferences", "description": "Teacher workload & availability"}
    ],
    "paths": {
        "/health": {
            "get": {
                "summary": "Health check",
                "responses": {
                    "200": {"description": "OK"}
                }
            }
        },
        "/ready": {
            "get": {
                "summary": "Readiness check",
                "responses": {
                    "200": {"description": "Ready"}
                }
            }
        },
        "/teachers": {
            "get": {
                "tags": ["Teachers"],
                "summary": "List teachers",
                "parameters": [
                    {"name": "search", "in": "query", "type": "string"},
                    {"name": "active", "in": "query", "type": "boolean"},
                    {"name": "page", "in": "query", "type": "integer"},
                    {"name": "limit", "in": "query", "type": "integer"},
                    {"name": "sort", "in": "query", "type": "string"},
                    {"name": "order", "in": "query", "type": "string"}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            },
            "post": {
                "tags": ["Teachers"],
                "summary": "Create teacher",
                "parameters": [
                    {"name": "payload", "in": "body", "required": true, "schema": {"$ref": "#/definitions/CreateTeacherRequest"}}
                ],
                "responses": {
                    "201": {"description": "Created", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            }
        },
        "/teachers/{id}": {
            "get": {
                "tags": ["Teachers"],
                "summary": "Get teacher",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            },
            "put": {
                "tags": ["Teachers"],
                "summary": "Update teacher",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"},
                    {"name": "payload", "in": "body", "required": true, "schema": {"$ref": "#/definitions/UpdateTeacherRequest"}}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            },
            "delete": {
                "tags": ["Teachers"],
                "summary": "Deactivate teacher",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"}
                ],
                "responses": {
                    "204": {"description": "No Content"}
                }
            }
        },
        "/teachers/{id}/assignments": {
            "get": {
                "tags": ["Teacher Assignments"],
                "summary": "List assignments",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            },
            "post": {
                "tags": ["Teacher Assignments"],
                "summary": "Create assignment",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"},
                    {"name": "payload", "in": "body", "required": true, "schema": {"$ref": "#/definitions/CreateTeacherAssignmentRequest"}}
                ],
                "responses": {
                    "201": {"description": "Created", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            }
        },
        "/teachers/{id}/assignments/{aid}": {
            "delete": {
                "tags": ["Teacher Assignments"],
                "summary": "Delete assignment",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"},
                    {"name": "aid", "in": "path", "required": true, "type": "string"}
                ],
                "responses": {
                    "204": {"description": "No Content"}
                }
            }
        },
        "/teachers/{id}/preferences": {
            "get": {
                "tags": ["Teacher Preferences"],
                "summary": "Get preferences",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            },
            "put": {
                "tags": ["Teacher Preferences"],
                "summary": "Upsert preferences",
                "parameters": [
                    {"name": "id", "in": "path", "required": true, "type": "string"},
                    {"name": "payload", "in": "body", "required": true, "schema": {"$ref": "#/definitions/UpsertTeacherPreferenceRequest"}}
                ],
                "responses": {
                    "200": {"description": "OK", "schema": {"$ref": "#/definitions/ResponseEnvelope"}}
                }
            }
        }
    },
    "definitions": {
        "Teacher": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "email": {"type": "string"},
                "full_name": {"type": "string"},
                "nip": {"type": "string"},
                "phone": {"type": "string"},
                "expertise": {"type": "string"},
                "active": {"type": "boolean"},
                "created_at": {"type": "string"},
                "updated_at": {"type": "string"}
            }
        },
        "TeacherAssignmentDetail": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "teacher_id": {"type": "string"},
                "class_id": {"type": "string"},
                "subject_id": {"type": "string"},
                "term_id": {"type": "string"},
                "created_at": {"type": "string"},
                "class_name": {"type": "string"},
                "subject_name": {"type": "string"},
                "term_name": {"type": "string"}
            }
        },
        "TeacherPreference": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "teacher_id": {"type": "string"},
                "max_load_per_day": {"type": "integer"},
                "max_load_per_week": {"type": "integer"},
                "unavailable": {
                    "type": "array",
                    "items": {"$ref": "#/definitions/TeacherUnavailableSlot"}
                },
                "created_at": {"type": "string"},
                "updated_at": {"type": "string"}
            }
        },
        "TeacherUnavailableSlot": {
            "type": "object",
            "properties": {
                "day_of_week": {"type": "string"},
                "time_range": {"type": "string"}
            }
        },
        "CreateTeacherRequest": {
            "type": "object",
            "properties": {
                "email": {"type": "string"},
                "full_name": {"type": "string"},
                "nip": {"type": "string"},
                "phone": {"type": "string"},
                "expertise": {"type": "string"}
            },
            "required": ["email", "full_name"]
        },
        "UpdateTeacherRequest": {
            "type": "object",
            "properties": {
                "email": {"type": "string"},
                "full_name": {"type": "string"},
                "nip": {"type": "string"},
                "phone": {"type": "string"},
                "expertise": {"type": "string"},
                "active": {"type": "boolean"}
            },
            "required": ["email", "full_name"]
        },
        "CreateTeacherAssignmentRequest": {
            "type": "object",
            "properties": {
                "class_id": {"type": "string"},
                "subject_id": {"type": "string"},
                "term_id": {"type": "string"}
            },
            "required": ["class_id", "subject_id", "term_id"]
        },
        "UpsertTeacherPreferenceRequest": {
            "type": "object",
            "properties": {
                "max_load_per_day": {"type": "integer"},
                "max_load_per_week": {"type": "integer"},
                "unavailable": {
                    "type": "array",
                    "items": {"$ref": "#/definitions/TeacherUnavailableSlot"}
                }
            }
        },
        "Pagination": {
            "type": "object",
            "properties": {
                "page": {"type": "integer"},
                "page_size": {"type": "integer"},
                "total_count": {"type": "integer"}
            }
        },
        "APIError": {
            "type": "object",
            "properties": {
                "code": {"type": "string"},
                "message": {"type": "string"},
                "status": {"type": "integer"}
            }
        },
        "ResponseEnvelope": {
            "type": "object",
            "properties": {
                "data": {"type": "object"},
                "error": {"$ref": "#/definitions/APIError"},
                "pagination": {"$ref": "#/definitions/Pagination"},
                "meta": {"type": "object"}
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
