# Home Monitoring

Cloud infrastructure for home monitoring on GCP.

## Prerequisites

- Go 1.23+
- OpenTofu 1.6+
- Docker
- gcloud CLI
- [Hurl](https://hurl.dev/) (for API tests)

## Quick Start

```bash
# Start local environment (emulators + API)
make up

# Run tests
make test-unit        # Unit tests only
make test-api         # API tests (requires 'make up')
make test-coverage    # Coverage report

# Stop environment
make down
```

## Local Development

```bash
# Run API locally with hot-reload (emulators in Docker, API native)
make dev

# Debug in VS Code: use "Debug telemetry-api" launch config
# Requires: go install github.com/go-delve/delve/cmd/dlv@latest
```

## GCP Setup (First Time)

```bash
gcloud auth login
gcloud auth application-default login
gcloud projects create home-monitoring --name="Home Monitoring"
gcloud config set project home-monitoring

# Enable billing at https://console.cloud.google.com/billing
# Set budget alert

```

## Deployment

```bash
# Initialize Terraform (first time)
make tf-init

# Deploy infrastructure (Cloud Run will fail first time - that's expected)
make tf-apply

# Build and push Docker image (requires Artifact Registry from above)
make docker-auth      # First time only
make docker-deploy

# Re-apply to create Cloud Run (now image exists)
make tf-apply
```

## Teardown

```bash
make tf-destroy
```

## API Endpoints

### Public (no auth)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check (Cloud Run requires this) |
| POST | `/auth/device` | Device login (credentials → token) |

### Device Endpoints (requires Firebase token)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/refresh` | Refresh auth token |
| POST | `/telemetry` | Ingest telemetry (JSON) |
| POST | `/telemetry/proto` | Ingest telemetry (protobuf) |
| GET | `/telemetry?limit=N` | Get device's telemetry |
| GET | `/commands?status=X` | Get device's pending commands |
| POST | `/commands/{id}/ack` | Acknowledge command |
| GET | `/devices/{id}` | Get device info (own only) |

### Admin Endpoints (requires GCP IAM identity token)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/admin/devices/provision` | Provision new device |
| POST | `/admin/devices/{id}/revoke` | Revoke device access |
| POST | `/admin/commands` | Create command for device |
| DELETE | `/admin/commands/{id}` | Delete command |
| POST | `/admin/schemas/{app}/{version}` | Upload measurement schema |
| GET | `/admin/schemas/{app}/{version}` | Get measurement schema |

## Device Authentication Flow

Uses Firebase Auth custom tokens (1-hour expiry).

```
┌──────────┐                        ┌─────────────┐
│  Device  │                        │  Cloud Run  │
└────┬─────┘                        └──────┬──────┘
     │                                     │
     │  1. POST /auth/device               │
     │     {device_id, secret}             │
     ├────────────────────────────────────>│
     │                                     │
     │  2. {token, expires_in: 3600}       │
     │<────────────────────────────────────┤
     │                                     │
     │  3. POST /telemetry/proto           │
     │     Authorization: Bearer <token>   │
     ├────────────────────────────────────>│
     │                                     │
```

### Admin Access (IAM-based)

Admin endpoints require a GCP identity token. Only emails in `provisioner_emails` (tfvars) can access.

```bash
# Add your email to terraform/environments/dev.tfvars:
# provisioner_emails = "your-email@gmail.com"

# Then redeploy
make docker-deploy && make tf-apply

# Now you can provision devices:
make provision-device
```

### Manual Provisioning

```bash
# Get identity token (requires gcloud auth login)
TOKEN=$(make identity-token 2>/dev/null)

# Or directly:
SERVICE_URL=$(gcloud run services describe telemetry-api --region us-west1 --format 'value(status.url)')
TOKEN=$(gcloud auth print-identity-token --audiences="$SERVICE_URL")

# Provision device (MAC → UUID mapping)
# MAC can be in any format: AA:BB:CC:DD:EE:FF, aa-bb-cc-dd-ee-ff, aabbccddeeff
curl -X POST $SERVICE_URL/admin/devices/provision \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mac_address":"AA:BB:CC:DD:EE:FF","app_name":"measurement-probe","app_version":"1.0.0"}'

# Response (save these!):
# {
#   "device_id": "550e8400-e29b-41d4-a716-446655440000",  <- UUID to store on device
#   "mac_address": "aabbccddeeff",                        <- Normalized MAC
#   "secret": "<64-char-hex>",                            <- Secret to store on device
#   "message": "..."
# }

# Upload schema (from firmware CI/CD)
curl -X POST $SERVICE_URL/schemas/measurement-probe/1.0.0 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"measurements":{"temperature":{"id":0,"name":"temperature","type":"float","unit":"°C"}}}'
```

### Device Runtime

```bash
# Device uses UUID (not MAC) for authentication
DEVICE_ID="550e8400-e29b-41d4-a716-446655440000"  # From provisioning
SECRET="<64-char-hex>"                              # From provisioning

# 1. Authenticate (get token)
TOKEN=$(curl -s -X POST $API_URL/auth/device \
  -H "Content-Type: application/json" \
  -d "{\"device_id\":\"$DEVICE_ID\",\"secret\":\"$SECRET\"}" | jq -r .token)

# 2. Send telemetry (with token)
curl -X POST $API_URL/telemetry/proto \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/octet-stream" \
  --data-binary @measurements.pb
```

## Project Structure

```
docs/
  architecture.md       # Infrastructure diagrams
  firmware-integration.md  # Firmware team API guide
terraform/           # Infrastructure (OpenTofu)
  environments/      # Environment configs (dev.tfvars)
  modules/           # cloud-run, firestore, pubsub, iam
services/
  telemetry-api/     # Go API service
tests/
  api/               # Hurl API tests
  integration/       # Go integration tests
```

## Configuration

All GCP config lives in `terraform/environments/dev.tfvars`. The Makefile reads from it automatically.

```bash
# Override for specific commands
make docker-deploy IMAGE_TAG=v1.0.0
make tf-apply ENV=prod  # Uses prod.tfvars
```
