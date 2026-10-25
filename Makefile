GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

help:
	@echo "This is a helper makefile for oapi-codegen"
	@echo "Targets:"
	@echo "    generate:             regenerate all generated files"
	@echo "    generate-libopenapi: regenerate with the libopenapi engine"
	@echo "    test:                 run all tests"
	@echo "    test-libopenapi:      run all tests with the libopenapi engine"
	@echo "    tidy         tidy go mod"
	@echo "    lint         lint the project"
	@echo "    install:     build and install the oapi-codegen-ifc7 binary"

$(GOBIN)/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/main/install.sh | sh -s -- -b $(GOBIN) v2.15.0

.PHONY: tools
tools: $(GOBIN)/golangci-lint

lint: tools
	# run the root module explicitly, to prevent recursive calls by re-invoking `make ...` top-level
	$(GOBIN)/golangci-lint run ./...
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && env GOBIN=$(GOBIN) make lint'

lint-ci: tools
	# for the root module, explicitly run the step, to prevent recursive calls
	$(GOBIN)/golangci-lint run ./... --output.text.path=stdout --timeout=5m
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && env GOBIN=$(GOBIN) make lint-ci'

generate:
	# for the root module, explicitly run the step, to prevent recursive calls
	go generate ./...
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make generate'

test:
	# for the root module, explicitly run the step, to prevent recursive calls
	go test -cover ./...
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make test'

.PHONY: generate-libopenapi
generate-libopenapi:
	OAPI_CODEGEN_ENGINE=libopenapi $(MAKE) generate

.PHONY: test-libopenapi
test-libopenapi:
	OAPI_CODEGEN_ENGINE=libopenapi $(MAKE) test

tidy:
	# for the root module, explicitly run the step, to prevent recursive calls
	go mod tidy
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make tidy'

# Install the local cmd as oapi-codegen-ifc7. Resolve the destination the same
# way `go install` does (GOBIN, otherwise the first GOPATH entry's bin), and
# do not use this Makefile's GOBIN, which points at ./bin for repo tools.
.PHONY: install
install:
	@dest="$$(go env GOBIN)"; \
	if [ -z "$$dest" ]; then dest="$$(go env GOPATH | cut -d: -f1)/bin"; fi; \
	mkdir -p "$$dest" && \
	go build -o "$$dest/oapi-codegen-ifc7" ./cmd/oapi-codegen && \
	echo "installed $$dest/oapi-codegen-ifc7"

$(GOBIN)/mdtoc:
	env GOBIN=$(GOBIN) go install sigs.k8s.io/mdtoc@v1.4.0

# Generate/update the Table of Contents in Markdown files.
# Files must contain <!-- toc --> / <!-- /toc --> sentinel comments.
# Use "make readme-toc-check" in CI to verify TOCs are up-to-date.
readme-toc: $(GOBIN)/mdtoc
	$(GOBIN)/mdtoc --inplace README.md

readme-toc-check: $(GOBIN)/mdtoc
	$(GOBIN)/mdtoc --inplace --dryrun README.md

tidy-ci:
	# for the root module, explicitly run the step, to prevent recursive calls
	go mod tidy -diff
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make tidy-ci'
