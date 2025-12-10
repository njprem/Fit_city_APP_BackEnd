# Self-Service Password Reset

## Functional Requirements
| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-PR-01 | Initiate Reset | System shall accept a reset request when a valid email is submitted. | High |
| FR-PR-02 | Generate OTP | System shall create a numeric OTP of configurable length for each request. | High |
| FR-PR-03 | Persist OTP Securely | System shall store OTP values hashed with salt and an expiration timestamp. | High |
| FR-PR-04 | Deliver OTP | System shall send the OTP to the user's registered email address. | High |
| FR-PR-05 | Confirm Reset | System shall validate OTP and new password input before updating the user credential. | High |
| FR-PR-06 | Invalidate Prior Tokens | System shall consume any existing reset tokens once a new one is issued or used. | Medium |

---

# Session Timeout

## Functional Requirements
| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-ST-01 | Issue Session | System shall generate a JWT and session record upon successful authentication. | High |
| FR-ST-02 | Validate Token | System shall verify JWT signature and expiry on every protected request. | High |
| FR-ST-03 | Check Session State | System shall ensure an active session row exists with a future expiry before allowing access. | High |
| FR-ST-04 | Logout | System shall deactivate the session token when the user logs out. | High |
| FR-ST-05 | Return Expiry | System shall expose the session expiration time to clients in authentication responses. | Medium |