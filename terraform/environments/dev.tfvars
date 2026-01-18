project_id            = "home-monitoring-483922"
region                = "us-west1"
environment           = "dev"
repository_id         = "home-monitoring"
allow_unauthenticated = true  # Required: devices use app-level auth, not GCP IAM

# Users allowed to impersonate the provisioner service account
provisioner_users = ["danielski.guilherme@gmail.com"]

# Set after first deploy, then re-run tofu apply
cloud_run_service_name = "telemetry-api"
# service_url is auto-detected from request Host header