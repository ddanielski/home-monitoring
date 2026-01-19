# Device Admin API tests
# Note: Admin endpoints require API key authentication

# POST /admin/devices/provision - requires admin API key
POST {{base_url}}/admin/devices/provision
Content-Type: application/json
{
  "mac_address": "AA:BB:CC:DD:EE:FF"
}
HTTP 401


# POST /admin/devices/{id}/revoke - requires admin API key
POST {{base_url}}/admin/devices/device-123/revoke
HTTP 401


# GET /devices/{id} - requires device auth (regular devices can only see own info)
GET {{base_url}}/devices/device-123
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"


# PUT /devices/info - requires device auth
PUT {{base_url}}/devices/info
Content-Type: application/json
{
  "app_name": "test-app",
  "app_version": "1.0.0"
}
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"


# PUT /devices/info - missing app_name
PUT {{base_url}}/devices/info
Content-Type: application/json
Authorization: Bearer mock-token
{
  "app_version": "1.0.0"
}
HTTP 401
# Note: Even with a token header, without valid Firebase auth this returns 401
