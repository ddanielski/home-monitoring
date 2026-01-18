# Telemetry API tests
# Note: All endpoints now require authentication

# POST telemetry - requires auth
POST {{base_url}}/telemetry
Content-Type: application/json
{
  "device_id": "sensor-001",
  "type": "temperature",
  "value": 23.5,
  "unit": "celsius"
}
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"


# GET telemetry - requires auth
GET {{base_url}}/telemetry
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"


# POST telemetry/proto - requires auth
POST {{base_url}}/telemetry/proto
Content-Type: application/octet-stream
```
test data
```
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"
