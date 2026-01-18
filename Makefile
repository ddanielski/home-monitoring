.PHONY: up down dev test test-unit test-integration test-api test-e2e test-coverage test-coverage-full test-infra deploy tf-init tf-plan tf-apply tf-destroy tf-validate tf-fmt ci ci-coverage docker-auth docker-build docker-push docker-deploy identity-token provision-device

# =============================================================================
# Configuration
# =============================================================================

# Environment (dev, prod) - determines which tfvars file to use
ENV ?= dev
TFVARS_FILE = terraform/environments/$(ENV).tfvars

# Local ports (for emulators)
API_PORT ?= 8000
FIRESTORE_PORT ?= 8080
PUBSUB_PORT ?= 8085
FIREBASE_AUTH_PORT ?= 9099

# GCP Configuration - read from tfvars (single source of truth)
GCP_PROJECT := $(shell grep 'project_id' $(TFVARS_FILE) 2>/dev/null | cut -d'"' -f2 || echo "home-monitoring")
GCP_REGION := $(shell grep 'region' $(TFVARS_FILE) 2>/dev/null | cut -d'"' -f2 || echo "us-west1")
REPOSITORY_ID := $(shell grep 'repository_id' $(TFVARS_FILE) 2>/dev/null | cut -d'"' -f2 || echo "home-monitoring")

# Use git SHA for immutable image tags (industry standard)
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "latest")
GIT_DIRTY := $(shell git status --porcelain 2>/dev/null | grep -q . && echo "-dirty" || echo "")
IMAGE_TAG := $(GIT_SHA)$(GIT_DIRTY)

# Derived - local
FIRESTORE_HOST = localhost:$(FIRESTORE_PORT)
PUBSUB_HOST = localhost:$(PUBSUB_PORT)
FIREBASE_AUTH_HOST = localhost:$(FIREBASE_AUTH_PORT)
API_URL = http://localhost:$(API_PORT)

# Derived - GCP
REGISTRY = $(GCP_REGION)-docker.pkg.dev/$(GCP_PROJECT)/$(REPOSITORY_ID)
IMAGE_NAME = telemetry-api
FULL_IMAGE = $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# =============================================================================
# Environment Management
# =============================================================================

up:
	docker compose --profile test up -d --build
	@echo "Test environment running:"
	@echo "  - Firestore:     $(FIRESTORE_HOST)"
	@echo "  - Pub/Sub:       $(PUBSUB_HOST)"
	@echo "  - Firebase Auth: $(FIREBASE_AUTH_HOST)"
	@echo "  - Firebase UI:   http://localhost:4000"
	@echo "  - API:           $(API_URL)"

down:
	docker compose --profile test down

# =============================================================================
# Development
# =============================================================================

dev:
	docker compose up -d
	@echo "Waiting for emulators..."
	@sleep 5
	cd services/telemetry-api && \
		PORT=$(API_PORT) \
		FIRESTORE_EMULATOR_HOST=$(FIRESTORE_HOST) \
		PUBSUB_EMULATOR_HOST=$(PUBSUB_HOST) \
		FIREBASE_AUTH_EMULATOR_HOST=$(FIREBASE_AUTH_HOST) \
		GCP_PROJECT=local-dev \
		go run .

# =============================================================================
# Tests - Run after 'make up'
# =============================================================================

test: test-unit test-infra
	@echo "Run 'make up' then 'make test-api' for API tests"

test-unit:
	cd services/telemetry-api && go test -short -v ./...

test-integration:
	cd services/telemetry-api && \
		FIRESTORE_EMULATOR_HOST=$(FIRESTORE_HOST) \
		PUBSUB_EMULATOR_HOST=$(PUBSUB_HOST) \
		FIREBASE_AUTH_EMULATOR_HOST=$(FIREBASE_AUTH_HOST) \
		go test -v ./...

test-api:
	hurl --test --variable base_url=$(API_URL) --variable timestamp=$$(date +%s) tests/api/*.hurl

test-e2e:
	cd tests/integration && API_BASE_URL=$(API_URL) go test -v ./...

test-coverage:
	cd services/telemetry-api && go clean -testcache
	cd services/telemetry-api && go test -short -coverprofile=coverage.out ./...
	cd services/telemetry-api && go tool cover -html=coverage.out -o coverage.html
	@echo ""
	@echo "=== Coverage Summary ==="
	@cd services/telemetry-api && go tool cover -func=coverage.out | grep -E "(commands\.go|telemetry\.go|handlers\.go|store_mock\.go|total)"
	@echo ""
	@echo "Note: store_firestore.go and store_pubsub.go require 'make up' first"
	@echo "Coverage report: services/telemetry-api/coverage.html"

test-coverage-full:
	cd services/telemetry-api && go clean -testcache
	cd services/telemetry-api && \
		FIRESTORE_EMULATOR_HOST=$(FIRESTORE_HOST) \
		PUBSUB_EMULATOR_HOST=$(PUBSUB_HOST) \
		FIREBASE_AUTH_EMULATOR_HOST=$(FIREBASE_AUTH_HOST) \
		go test -coverprofile=coverage.out ./...
	cd services/telemetry-api && go tool cover -html=coverage.out -o coverage.html
	@echo ""
	@echo "=== Coverage Summary (with emulators) ==="
	@cd services/telemetry-api && go tool cover -func=coverage.out | tail -10
	@echo ""
	@echo "Coverage report: services/telemetry-api/coverage.html"

# =============================================================================
# Infrastructure Tests
# =============================================================================

test-infra: tf-validate tf-fmt-check
	@echo "Infrastructure tests passed"

tf-validate:
	cd terraform && tofu validate

tf-fmt-check:
	cd terraform && tofu fmt -check -recursive

tf-fmt:
	cd terraform && tofu fmt -recursive

# =============================================================================
# Device Provisioning (requires gcloud auth login)
# =============================================================================

# Provisioner service account (derived from project)
PROVISIONER_SA = provisioner@$(GCP_PROJECT).iam.gserviceaccount.com

# Get identity token for calling admin endpoints (via SA impersonation)
identity-token:
	@service_url=$$(gcloud run services describe telemetry-api --region $(GCP_REGION) --project $(GCP_PROJECT) --format 'value(status.url)' 2>/dev/null) && \
	gcloud auth print-identity-token \
		--impersonate-service-account=$(PROVISIONER_SA) \
		--audiences="$$service_url"

# Provision a new device (returns UUID + secret to store on device)
provision-device:
	@service_url=$$(gcloud run services describe telemetry-api --region $(GCP_REGION) --project $(GCP_PROJECT) --format 'value(status.url)') && \
	echo "Service URL: $$service_url" && \
	echo "Provisioner SA: $(PROVISIONER_SA)" && \
	read -p "Enter MAC address (e.g., AA:BB:CC:DD:EE:FF): " mac_address && \
	echo "Getting identity token via SA impersonation..." && \
	token=$$(gcloud auth print-identity-token \
		--impersonate-service-account=$(PROVISIONER_SA) \
		--audiences="$$service_url") && \
	echo "Provisioning device..." && \
	curl -s -X POST "$$service_url/admin/devices/provision" \
		-H "Authorization: Bearer $$token" \
		-H "Content-Type: application/json" \
		-d "{\"mac_address\": \"$$mac_address\"}" | jq .

# =============================================================================
# Infrastructure Management
# =============================================================================

tf-init:
	cd terraform && tofu init

tf-plan:
	cd terraform && tofu plan -var-file=environments/$(ENV).tfvars

tf-apply:
	@echo "Deploying image tag: $(IMAGE_TAG)"
	cd terraform && tofu apply -var-file=environments/$(ENV).tfvars -var="image_tag=$(IMAGE_TAG)"

tf-destroy:
	cd terraform && tofu destroy -var-file=environments/$(ENV).tfvars

# =============================================================================
# Docker Image Management
# =============================================================================

docker-auth:
	gcloud auth configure-docker $(GCP_REGION)-docker.pkg.dev

# Check for uncommitted changes
check-clean:
	@if [ -n "$(GIT_DIRTY)" ]; then \
		echo "⚠️  WARNING: Uncommitted changes detected!"; \
		echo "   Image will be tagged: $(IMAGE_TAG)"; \
		echo "   Consider committing first for reproducible builds."; \
		echo ""; \
		read -p "Continue anyway? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1; \
	fi

docker-build: check-clean
	docker build -t $(IMAGE_NAME) services/telemetry-api/
	docker tag $(IMAGE_NAME) $(FULL_IMAGE)
	@echo "Built: $(FULL_IMAGE)"

docker-build-clean: check-clean
	docker build --no-cache -t $(IMAGE_NAME) services/telemetry-api/
	docker tag $(IMAGE_NAME) $(FULL_IMAGE)
	@echo "Built (no-cache): $(FULL_IMAGE)"

docker-push: docker-build
	docker push $(FULL_IMAGE)
	@echo "Pushed: $(FULL_IMAGE)"

docker-deploy: docker-push
	@echo "Image deployed to: $(FULL_IMAGE)"
	@echo "Run 'make tf-apply' to deploy to Cloud Run"

docker-deploy-clean: docker-build-clean
	docker push $(FULL_IMAGE)
	@echo "Image deployed (no-cache): $(FULL_IMAGE)"
	@echo "Run 'make tf-apply' to deploy to Cloud Run"

# =============================================================================
# Deployment
# =============================================================================

deploy:
	go run ./scripts/deploy --project $(GCP_PROJECT)

# =============================================================================
# CI - Full test cycle with container management
# =============================================================================

ci: up
	@sleep 5
	$(MAKE) test-unit
	$(MAKE) test-infra
	$(MAKE) test-api
	$(MAKE) test-e2e
	$(MAKE) down

ci-coverage: up
	@sleep 5
	$(MAKE) test-coverage-full
	$(MAKE) down
