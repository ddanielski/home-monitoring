project_id            = "home-monitoring-483922"
region                = "us-west1"
environment           = "dev"
repository_id         = "home-monitoring"
allow_unauthenticated = true  # Required: devices use app-level auth, not GCP IAM

# Users allowed to access the admin API key from Secret Manager
provisioner_users = ["danielski.guilherme@gmail.com"]