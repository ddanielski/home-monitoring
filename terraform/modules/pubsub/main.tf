variable "project_id" {
  description = "GCP project ID"
  type        = string
}

resource "google_pubsub_topic" "telemetry" {
  name = "telemetry-events"

  message_retention_duration = "86400s"

  labels = {
    purpose = "telemetry-ingestion"
  }
}

resource "google_pubsub_topic" "telemetry_dlq" {
  name = "telemetry-events-dlq"

  labels = {
    purpose = "dead-letter-queue"
  }
}

resource "google_pubsub_subscription" "telemetry_processor" {
  name  = "telemetry-processor"
  topic = google_pubsub_topic.telemetry.id

  ack_deadline_seconds = 30

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.telemetry_dlq.id
    max_delivery_attempts = 5
  }

  message_retention_duration = "604800s"

  expiration_policy {
    ttl = ""
  }
}

resource "google_pubsub_topic" "commands" {
  name = "device-commands"

  message_retention_duration = "86400s"

  labels = {
    purpose = "device-commands"
  }
}

output "telemetry_topic_name" {
  description = "Name of the telemetry Pub/Sub topic"
  value       = google_pubsub_topic.telemetry.name
}

output "telemetry_topic_id" {
  description = "ID of the telemetry Pub/Sub topic"
  value       = google_pubsub_topic.telemetry.id
}

output "commands_topic_name" {
  description = "Name of the commands Pub/Sub topic"
  value       = google_pubsub_topic.commands.name
}

output "commands_topic_id" {
  description = "ID of the commands Pub/Sub topic"
  value       = google_pubsub_topic.commands.id
}
