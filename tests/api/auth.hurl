# Authentication API tests

# POST /auth/device - invalid credentials (device doesn't exist)
POST {{base_url}}/auth/device
Content-Type: application/json
{
  "device_id": "nonexistent-device",
  "secret": "invalid-secret"
}
HTTP 401
[Asserts]
jsonpath "$.error" == "invalid credentials"


# POST /auth/device - missing device_id
POST {{base_url}}/auth/device
Content-Type: application/json
{
  "secret": "some-secret"
}
HTTP 400
[Asserts]
jsonpath "$.error" == "device_id is required"


# POST /auth/device - missing secret
POST {{base_url}}/auth/device
Content-Type: application/json
{
  "device_id": "device-123"
}
HTTP 400
[Asserts]
jsonpath "$.error" == "secret is required"


# POST /auth/device - invalid JSON
POST {{base_url}}/auth/device
Content-Type: application/json
```
{invalid json}
```
HTTP 400
[Asserts]
jsonpath "$.error" == "invalid JSON"


# POST /auth/refresh - requires valid token
POST {{base_url}}/auth/refresh
HTTP 401
[Asserts]
jsonpath "$.error" contains "authorization"
