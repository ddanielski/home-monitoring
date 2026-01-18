# API flow test - tests public endpoints and auth requirements

# 1. Health check (public)
GET {{base_url}}/health
HTTP 200
[Asserts]
jsonpath "$.status" == "healthy"


# 2. Auth device with missing credentials
POST {{base_url}}/auth/device
Content-Type: application/json
{}
HTTP 400
[Asserts]
jsonpath "$.error" contains "device_id"


# 3. Auth device with missing secret
POST {{base_url}}/auth/device
Content-Type: application/json
{
  "device_id": "test-device-001"
}
HTTP 400
[Asserts]
jsonpath "$.error" contains "secret"


# 4. Auth device with invalid credentials
POST {{base_url}}/auth/device
Content-Type: application/json
{
  "device_id": "nonexistent-device",
  "secret": "invalid-secret"
}
HTTP 401
[Asserts]
jsonpath "$.error" contains "invalid"


# 5. Protected endpoints return 401 without token
GET {{base_url}}/telemetry
HTTP 401

GET {{base_url}}/commands
HTTP 401

GET {{base_url}}/devices/test-device
HTTP 401


# 6. Admin endpoints return 401 without IAM token
POST {{base_url}}/admin/devices/provision
Content-Type: application/json
{
  "mac_address": "AA:BB:CC:DD:EE:FF",
  "app_name": "test-app",
  "app_version": "1.0.0"
}
HTTP 401
