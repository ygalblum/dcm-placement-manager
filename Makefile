BINARY_NAME := placement-manager
# COMPOSE: compose command. Set to override; otherwise auto-detect podman-compose or docker-compose.
COMPOSE ?= $(shell command -v podman-compose >/dev/null 2>&1 && echo podman-compose || \
	(command -v docker-compose >/dev/null 2>&1 && echo docker-compose || \
	(echo "docker compose")))

build:
	go build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

run:
	go run ./cmd/$(BINARY_NAME)

clean:
	rm -rf bin/

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --fail-on-pending --skip-package=test/subsystem

tidy:
	go mod tidy

generate-types:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/types.gen.cfg \
		-o api/v1alpha1/types.gen.go \
		api/v1alpha1/openapi.yaml

generate-spec:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/spec.gen.cfg \
		-o api/v1alpha1/spec.gen.go \
		api/v1alpha1/openapi.yaml

generate-server:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=internal/api/server/server.gen.cfg \
		-o internal/api/server/server.gen.go \
		api/v1alpha1/openapi.yaml

generate-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=pkg/client/client.gen.cfg \
		-o pkg/client/client.gen.go \
		api/v1alpha1/openapi.yaml

generate-api: generate-types generate-spec generate-server generate-client

check-generate-api: generate-api
	git diff --exit-code api/ internal/api/server/ pkg/client/ || \
		(echo "Generated files out of sync. Run 'make generate-api'." && exit 1)

# Check AEP compliance
check-aep:
	spectral lint --fail-severity=warn ./api/v1alpha1/openapi.yaml

subsystem-test-up:
	$(COMPOSE) -f test/subsystem/docker-compose.yaml up -d --build

subsystem-test-down:
	$(COMPOSE) -f test/subsystem/docker-compose.yaml down -v

subsystem-test:
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --fail-on-pending -tags=subsystem ./test/subsystem

subsystem-test-full: subsystem-test-up subsystem-test subsystem-test-down

.PHONY: build run clean fmt vet test tidy generate-types generate-spec generate-server generate-client generate-api check-aep check-generate-api subsystem-test-up subsystem-test-down subsystem-test subsystem-test-full
