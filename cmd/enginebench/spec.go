package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeSpec writes a deterministic OpenAPI 3.0 document with resources
// paths. Each path has a get and a post, and each schema references the
// next, so both engines resolve a large, cyclic document.
func writeSpec(dir string, resources int) (string, error) {
	if resources < 1 {
		return "", fmt.Errorf("resources must be at least 1")
	}
	path := filepath.Join(dir, "spec.yaml")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(f, `openapi: 3.0.3
info:
  title: Engine benchmark
  version: "1.0.0"
paths:
`)
	for i := 0; i < resources; i++ {
		fmt.Fprintf(f, `  "/groups/%d/items/{id}":
    get:
      operationId: getItem%d
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Item%d"
    post:
      operationId: createItem%d
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Item%d"
      responses:
        "201":
          description: created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Item%d"
`, i, i, i, i, i, i)
	}
	fmt.Fprintf(f, "components:\n  schemas:\n")
	for i := 0; i < resources; i++ {
		next := (i + 1) % resources
		fmt.Fprintf(f, `    Item%d:
      type: object
      required: [id, name]
      properties:
        id:
          type: string
        name:
          type: string
        count:
          type: integer
          format: int32
        active:
          type: boolean
        tag:
          type: string
          enum: [alpha, beta, gamma]
        related:
          $ref: "#/components/schemas/Item%d"
        labels:
          type: array
          items:
            type: string
`, i, next)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, nil
}
