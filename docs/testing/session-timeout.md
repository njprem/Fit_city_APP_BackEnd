# Test Cases for Session Timeout

## 1. Functional Test Cases

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ST-001 | Issue Session | Verify that a session is created upon successful login. | 1. Authenticate with valid credentials via `POST /api/v1/auth/login`. | The system returns a JWT, the user object, and an `expires_at` timestamp. A new record is created in the `sessions` table with `is_active=true`. |
| TC-ST-002 | Validate Token | Verify that a valid JWT grants access to protected routes. | 1. Log in to obtain a valid JWT. <br> 2. Make a request to a protected endpoint (e.g., `GET /api/v1/auth/me`) with the JWT in the `Authorization` header. | The request is successful (200 OK), and the endpoint returns the expected data. |
| TC-ST-003 | Expired Token | Verify that an expired JWT is rejected. | 1. Log in to obtain a valid JWT. <br> 2. Wait for the session to expire (e.g., based on `SESSION_TTL`). <br> 3. Make a request to a protected endpoint with the expired JWT. | The system returns a 401 Unauthorized error. |
| TC-ST-004 | Logout | Verify that logging out deactivates the session. | 1. Log in to obtain a valid JWT. <br> 2. Call `POST /api/v1/auth/logout` with the JWT. <br> 3. Attempt to use the same JWT to access a protected endpoint. | The logout request is successful. The subsequent request to the protected endpoint is rejected with a 401 Unauthorized error. The `is_active` flag for the session in the database is set to `false`. |
| TC-ST-005 | Invalid Token | Verify that a malformed or invalid JWT is rejected. | 1. Make a request to a protected endpoint with a JWT that is malformed or has an invalid signature. | The system returns a 401 Unauthorized error. |
| TC-ST-006 | Session Expiry | Verify that a session expires based on the `expires_at` field in the database. | 1. Log in to obtain a valid JWT. <br> 2. Manually update the `expires_at` field in the `sessions` table for the current session to a time in the past. <br> 3. Make a request to a protected endpoint with the still-valid JWT. | The system returns a 401 Unauthorized error because the backing session in the database is expired. |
| TC-ST-007 | Multiple Sessions | Verify that a user can have multiple active sessions. | 1. Log in on one device/client and obtain a JWT (Token A). <br> 2. Log in on a second device/client and obtain another JWT (Token B). <br> 3. Verify that both Token A and Token B can be used to access protected routes. <br> 4. Log out from the first device (using Token A). <br> 5. Verify that Token B is still valid and Token A is not. | Both tokens are initially valid. After logging out with Token A, it becomes invalid, but Token B remains valid. |

## 2. Non-Functional Test Cases

| Test Case ID | Category | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ST-NFR-001 | Security | Verify that the JWT is signed and the signature is validated. | 1. Obtain a valid JWT. <br> 2. Tamper with the payload of the JWT without re-signing it. <br> 3. Use the tampered token to access a protected endpoint. | The system returns a 401 Unauthorized error due to the invalid signature. |
| TC-ST-NFR-002 | Performance | Verify that token validation is performant. | 1. Use a performance testing tool to send a high volume of requests with valid tokens to a protected endpoint. | The average response time for token validation should be within the acceptable range (e.g., under 100ms). |
| TC-ST-NFR-003 | Reliability | Verify that the system handles database connection errors during session validation. | 1. Simulate a database connection failure. <br> 2. Attempt to access a protected endpoint with a valid JWT. | The system should return a 5xx error, indicating a server-side issue, and should not grant access. |
| TC-ST-NFR-004 | Scalability | Verify that session lookups are efficient. | 1. Populate the `sessions` table with a large number of records. <br> 2. Measure the performance of session lookups during authentication. | The lookup time should not degrade significantly, demonstrating that the queries are using indexes effectively. |
