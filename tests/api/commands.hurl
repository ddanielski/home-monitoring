# Commands API tests
# Note: All endpoints now require authentication

# GET commands - requires device auth
GET {{base_url}}/commands
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"


# POST commands/{id}/ack - requires device auth
POST {{base_url}}/commands/cmd-123/ack
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"


# Admin endpoints require IAM auth

# POST admin/commands - requires IAM auth
POST {{base_url}}/admin/commands
Content-Type: application/json
{
  "device_id": "sensor-001",
  "type": "reboot"
}
HTTP 401


# DELETE admin/commands - requires IAM auth
DELETE {{base_url}}/admin/commands/cmd-123
HTTP 401
