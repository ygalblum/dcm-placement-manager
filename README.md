# DCM Placement Manager

DCM Placement Manager is a Go service that orchestrates resource provisioning within the DCM ecosystem.
On receiving a creation request, the DCM Placement Manager evaluates the spec against
the [Policy Engine](https://github.com/dcm-project/policy-manager),
persists the resource, and forwards it to
the [Service Provider Resource Manager (SPRM)](https://github.com/dcm-project/service-provider-manager)
for provisioning.

The API follows [AEP (API Enhancement Proposals)](https://aep.dev/) standards for resource-oriented design
and uses [RFC 7807](https://tools.ietf.org/html/rfc7807) Problem Details for error responses.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Building](#building)
    - [Running Locally](#running-locally)
- [API](#api)
- [Configuration](#configuration)
- [Development Guide](#development-guide)
    - [Project Structure](#project-structure)
    - [Code Generation](#code-generation)
    - [Testing](#testing)
    - [AEP Compliance](#aep-compliance)
    - [CI/CD](#cicd)
- [License](#license)

## Architecture Overview

```
                    ┌──────────────────────────────────────────┐
                    │           Placement Manager              │
  Create / Get /    │                                          │
  List / Delete     │  ┌──────────┐  ┌──────────┐              │       ┌──────────────┐
  (port 8080)  ────>│  │ Handlers │──│ Service  │───────────── │──────>│  PostgreSQL  │
                    │  └──────────┘  └──────────┘              │       └──────────────┘
                    │                     │                    │
                    │              ┌──────┴──────┐             │
                    │              │             │             │
                    │              ▼             ▼             │
                    │     ┌──────────────┐ ┌──────────┐        │
                    │     │Policy Engine │ │   SPRM   │        │
                    │     └──────────────┘ └──────────┘        │
                    └──────────────────────────────────────────┘
                              │                  │
                              ▼                  ▼
                    ┌──────────────────┐  ┌──────────────────┐
                    │  Policy Manager  │  │ Service Provider │
                    │   (port 8081)    │  │ Resource Manager │
                    │                  │  │   (port 8082)    │
                    └──────────────────┘  └──────────────────┘
```

The service follows a three-layer architecture:  
**Handler** (HTTP concerns) → **Service** (business logic) → **Store** (data access via GORM).

### Resource Creation Flow

1. Client sends a `POST /resources` request with the value of `catalog_item_instance_id` and `spec`.
2. The service evaluates the spec against the **Policy Engine**. The policy returns an approval status (`APPROVED` or
   `MODIFIED`), a selected provider, and an optionally modified spec.
3. The resource is persisted in the database with the **original** spec, approval status, and provider name.
4. The **evaluated** spec (potentially modified by policy) is forwarded to the **SPRM** for provisioning.
5. If SPRM provisioning fails, the database record is rolled back.

### Resource Deletion Flow

1. The service retrieves the resource from the database to obtain the 
   value of `catalog_item_instance_id`.
2. The resource is deleted from the **SPRM** first.
3. If the SPRM deletion succeeds, the database record is removed.
4. If the SPRM deletion fails, the database record is preserved.

## Getting Started

### Prerequisites

- **Go** 1.25.5+
- **PostgreSQL** 16+ (or SQLite for development)
- **Podman** or **Docker** with Compose (for containerized setup)

### Building

```bash
make build          # Build binary to bin/placement-manager
make fmt            # Format code
make vet            # Run go vet
make tidy           # Tidy module dependencies
```

### Running Locally

1. Start PostgreSQL (or use SQLite by setting `DB_TYPE=sqlite`), the Policy Manager, and the SPRM.

2. Set environment variables (see [Configuration](#configuration)):

```bash
export DB_TYPE=pgsql
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=placement-manager
export DB_USER=admin
export DB_PASSWORD=adminpass
export POLICY_MANAGER_EVALUATION_URL=http://localhost:8081
export SP_RESOURCE_MANAGER_URL=http://localhost:8082
```

3. Run the service:

```bash
make run
```

4. Verify:

```bash
curl http://localhost:8080/api/v1alpha1/health
# {"status":"healthy","path":"health"}
```

## API

The API is served at `/api/v1alpha1` and follows AEP resource-oriented design
with RFC 7807 error responses.

Full OpenAPI specification: [`api/v1alpha1/openapi.yaml`](api/v1alpha1/openapi.yaml)

### Endpoints

| Method   | Path                    | Description                                          |
|----------|-------------------------|------------------------------------------------------|
| `GET`    | `/health`               | Health check                                         |
| `GET`    | `/resources`            | List resources (paginated, filterable by `provider`)  |
| `POST`   | `/resources`            | Create a resource                                    |
| `GET`    | `/resources/{id}`       | Get a resource                                       |
| `DELETE` | `/resources/{id}`       | Delete a resource                                    |

### Resource IDs

All resources support optional user-specified IDs via the `id` query parameter
(DNS-1123 format: lowercase alphanumeric with hyphens, max 63 characters).

### Key Features

- **Pagination** — Token-based pagination with configurable page size (default:
  100, max: 100).
- **Policy Evaluation** — On creation, the spec is evaluated by the policy
  engine which returns an approval status (`APPROVED` or `MODIFIED`), a selected
  provider, and an optionally modified spec.
- **SPRM Provisioning** — The evaluated spec is forwarded to the Service Provider
  Resource Manager. If provisioning fails, the database record is rolled back.
- **Deletion Safety** — On delete, the SPRM resource is removed first. If SPRM
  deletion fails, the database record is preserved.
- **Retry with Backoff** — Both the policy engine and SPRM clients use
  exponential backoff with configurable timeouts.

## Configuration

All configuration is via environment variables:

| Variable                            | Default                 | Description                        |
|-------------------------------------|-------------------------|------------------------------------|
| `SVC_ADDRESS`                       | `:8080`                 | HTTP server listen address         |
| `SVC_LOG_LEVEL`                     | `info`                  | Logging level                      |
| `DB_TYPE`                           | `pgsql`                 | Database type: `pgsql` or `sqlite` |
| `DB_HOST`                           | `localhost`             | PostgreSQL hostname                |
| `DB_PORT`                           | `5432`                  | PostgreSQL port                    |
| `DB_NAME`                           | `placement-manager`     | Database name                      |
| `DB_USER`                           | *(none)*                | Database user                      |
| `DB_PASSWORD`                       | *(none)*                | Database password                  |
| `POLICY_MANAGER_EVALUATION_URL`     | `http://localhost:8081` | Policy engine base URL             |
| `POLICY_MANAGER_EVALUATION_TIMEOUT` | `10s`                   | Policy engine request timeout      |
| `SP_RESOURCE_MANAGER_URL`           | `http://localhost:8082` | SPRM base URL                      |
| `SP_RESOURCE_MANAGER_TIMEOUT`       | `10s`                   | SPRM request timeout               |

## Development Guide

### Project Structure

```
placement-manager/
├── api/v1alpha1/                    # OpenAPI spec and generated types
│   ├── openapi.yaml                 # API specification (source of truth)
│   ├── types.gen.go                 # Generated Go types
│   └── spec.gen.go                  # Generated embedded spec
├── cmd/placement-manager/
│   └── main.go                      # Application entry point
├── internal/
│   ├── api/server/                  # Generated Chi server stubs
│   ├── apiserver/                   # HTTP server setup and middleware
│   ├── config/                      # Environment variable configuration
│   ├── handlers/                    # HTTP handlers and error mapping
│   ├── httputil/                    # Shared HTTP retry and backoff utilities
│   ├── policy/                      # Policy engine client (with retry)
│   ├── service/                     # Business logic, model conversion, errors
│   ├── sprm/                        # SPRM client (with retry)
│   └── store/                       # GORM data access layer
│       └── model/                   # Database models
├── pkg/client/                      # Generated Go client library
├── test/subsystem/                  # Subsystem integration tests
│   ├── docker-compose.yaml          # PostgreSQL + WireMock services
│   ├── suite_test.go                # Test suite entry point
│   ├── setup_test.go                # WireMock helpers
│   └── placement_test.go            # Resource CRUD integration tests
├── Containerfile                    # Multi-stage container build
├── Makefile                         # Build and test targets
└── tools.go                         # Build tool dependencies
```

### Code Generation

The project uses [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) 
to generate Go types, server stubs, and client code from the OpenAPI specification.
After modifying `api/v1alpha1/openapi.yaml`, regenerate the code:

```bash
make generate-api           # Regenerate all API code

# Or generate specific components:
make generate-types         # API types
make generate-spec          # Embedded spec
make generate-server        # Chi server stubs
make generate-client        # Client library

# Verify generated files are in sync:
make check-generate-api
```

### Testing

The project uses [Ginkgo](https://onsi.github.io/ginkgo/) as the test framework
with [Gomega](https://onsi.github.io/gomega/) matchers.

#### Unit Tests

```bash
make test                   # Run all unit tests (excludes subsystem)
```

Test suites cover:

- **Handler tests** — HTTP request/response mapping with mock services
- **Service tests** — Business logic with mock policy and SPRM clients
- **Store tests** — Database operations with in-memory SQLite
- **Policy client tests** — HTTP client behavior, retries, and timeouts
- **SPRM client tests** — HTTP client behavior, retries, and timeouts

#### Subsystem Tests

Subsystem tests run the Placement Manager in a container with a real PostgreSQL 
database and WireMock stubs for the Policy Manager and SPRM. 
They require Podman or Docker with Compose.

```bash
# Full cycle (start services, run tests, stop services)
make subsystem-test-full

# Or step-by-step:
make subsystem-test-up      # Start services
make subsystem-test         # Run subsystem tests
make subsystem-test-down    # Stop and clean up
```

Subsystem tests use the `subsystem` build tag and read the following environment variables:

- `PLACEMENT_MANAGER_URL` (default: `http://localhost:28080`)
- `POLICY_MANAGER_EVALUATION_URL` (default: `http://localhost:28081`)
- `SP_RESOURCE_MANAGER_URL` (default: `http://localhost:28082`)

### AEP Compliance

The API specification is validated against [AEP standards](https://aep.dev/)
using [Spectral](https://stoplight.io/spectral):

```bash
make check-aep
```

The linter configuration is in `.spectral.yaml`.

### CI/CD

GitHub Actions workflows:

| Workflow               | Trigger              | Purpose                          |
|------------------------|----------------------|----------------------------------|
| `ci.yaml`              | All PRs to main      | Build and test                   |
| `check-generate.yaml`  | API file changes     | Verify generated code is in sync |
| `check-aep.yaml`       | OpenAPI spec changes | AEP standards compliance         |
| `build-push-quay.yaml` | Releases             | Build and push container image   |

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
