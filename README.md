# Home Monitoring Infrastructure

Cloud infrastructure and automation scripts for home monitoring systems.

## Structure

```
.
├── cmd/                    # Go CLI tools and scripts
├── internal/               # Shared Go packages
├── infra/                  # Infrastructure as Code (Terraform)
│   ├── modules/            # Reusable Terraform modules
│   └── environments/       # Environment-specific configurations
├── scripts/                # Shell scripts and utilities
└── docs/                   # Documentation
```

## Prerequisites

- Go 1.23+
- Terraform 1.5+
- Cloud provider CLI (aws/gcloud/az)

## Getting Started

```bash
# Initialize Go dependencies
go mod tidy

# Initialize Terraform
cd infra/environments/dev
terraform init
```

## Scripts

Build and run Go scripts:

```bash
go run ./cmd/<script-name>
```

## License

MIT
