# Petstore Expanded Example

This example demonstrates [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) generating a `net/http` server from an [OpenAPI 3.0 spec](petstore-expanded.yaml), backed by a trivial in-memory database (a Go map). A test client (`common/client`) exercises the server over real HTTP.

## Directory Structure

```
petstore-expanded/
├── petstore-expanded.yaml          # OpenAPI spec
├── petstore.go                     # main() wiring (thin wrapper around server/setup.go)
├── api/
│   ├── server.cfg.yaml             # Codegen config: server, models and embedded spec
│   ├── generate.go                 # go:generate for the server
│   └── petstore-server.gen.go      # Generated server and model types
├── server/
│   ├── store.go                    # In-memory CRUD store
│   ├── server.go                   # Hand-written ServerInterface implementation
│   └── setup.go                    # Builds the configured *http.Server
└── common/
    └── client/                     # Test client
        ├── main.go                 # CLI test client
        ├── testclient/
        │   └── testclient.go       # Reusable CRUD test sequence
        └── openapi/
            ├── generate.go         # go:generate for the client
            ├── client.cfg.yaml     # Codegen config: client + models
            └── client.gen.go       # Generated HTTP client (+ model types)
```

## Generating Code

From the repository root, `make generate` regenerates everything. To regenerate a
single piece from the `examples/` directory:

```sh
# Generate the server (server + models)
cd examples/petstore-expanded/api && go generate ./...

# Generate the test client
cd examples/petstore-expanded/common/client/openapi && go generate ./...
```

## Running the Server

```sh
cd examples/petstore-expanded
go run . --port 8080
```

## Test Client

The client executable verifies the server's behavior over real HTTP.
Start the server in one terminal, then point the client at it from another:

```sh
# Terminal 1: start the server
cd examples/petstore-expanded && go run . --port 8080

# Terminal 2: run the test client against it
cd examples && go run ./petstore-expanded/common/client/ --port 8080
```

The client verifies: add pets, find by ID, 404 on missing pet, list/filter by tag, delete, and empty list after deletion.
