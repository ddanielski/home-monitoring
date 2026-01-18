# Device Admin API tests
# Note: Admin endpoints require IAM authentication (gcloud identity tokens)

# POST /admin/devices/provision - requires IAM auth
POST {{base_url}}/admin/devices/provision
Content-Type: application/json
{
  "mac_address": "AA:BB:CC:DD:EE:FF",
  "app_name": "test-app",
  "app_version": "1.0.0"
}
HTTP 401


# POST /admin/devices/{id}/revoke - requires IAM auth
POST {{base_url}}/admin/devices/device-123/revoke
HTTP 401


# GET /devices/{id} - requires device auth (regular devices can only see own info)
GET {{base_url}}/devices/device-123
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"
