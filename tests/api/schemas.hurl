# Schema API tests
# Note: All schema endpoints are admin-only (IAM auth required)

# GET /admin/schemas/{app}/{version} - requires IAM auth
GET {{base_url}}/admin/schemas/test-app/1.0.0
HTTP 401


# POST /admin/schemas/{app}/{version} - requires IAM auth
POST {{base_url}}/admin/schemas/test-app/1.0.0
Content-Type: application/json
{
  "measurements": {
    "temperature": {"id": 1, "name": "temperature", "type": "float", "unit": "celsius"}
  }
}
HTTP 401
