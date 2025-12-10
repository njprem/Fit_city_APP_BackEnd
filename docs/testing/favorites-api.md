# Test Cases for Favorites API

## 1. Functional Test Cases

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-FAV-001 | Save a Destination | Verify an authenticated user can save a destination as a favorite. | 1. As an authenticated user, make a `POST` request to `/api/v1/users/me/favorites` with a valid `destination_id`. | The API returns a 201 Created status with a success message and the new favorite's details. A record is created in the `favorite_list` table. |
| TC-FAV-002 | Unsave a Destination | Verify an authenticated user can remove a destination from their favorites. | 1. Save a destination as a favorite. <br> 2. Make a `DELETE` request to `/api/v1/users/me/favorites/{destinationId}`. | The API returns a 200 OK status with a success message. The corresponding record is removed from the `favorite_list` table. |
| TC-FAV-003 | List Favorites | Verify an authenticated user can retrieve their list of saved destinations. | 1. Save several destinations as favorites. <br> 2. Make a `GET` request to `/api/v1/users/me/favorites`. | The API returns a 200 OK status with a paginated list of the user's favorite destinations, ordered by `saved_at` descending. |
| TC-FAV-004 | Count Favorites | Verify that the system can count how many users have favorited a destination. | 1. Have multiple users favorite the same destination. <br> 2. Make a `GET` request to `/api/v1/destinations/{destinationId}/favorites/count`. | The API returns a 200 OK status with the correct `favorites_count`. |
| TC-FAV-005 | Prevent Duplicate Saves | Verify the system prevents a user from saving the same destination twice. | 1. Save a destination as a favorite. <br> 2. Attempt to save the same destination again with a `POST` request. | The API returns a 409 Conflict error with a message like `"already_saved"`. No new record is created. |
| TC-FAV-006 | Unsave a Non-Favorited Item | Verify the system handles requests to unsave a destination that isn't a favorite. | 1. Choose a destination that the user has not favorited. <br> 2. Make a `DELETE` request to `/api/v1/users/me/favorites/{destinationId}`. | The API returns a 404 Not Found error with a message like `"not_saved"`. |
| TC-FAV-007 | List Empty Favorites | Verify the API returns an empty list for a user with no favorites. | 1. As a user who has not saved any favorites, make a `GET` request to `/api/v1/users/me/favorites`. | The API returns a 200 OK status with an empty `items` array. |
| TC-FAV-008 | Pagination of Favorites | Verify pagination of the favorites list. | 1. Save more than 20 destinations. <br> 2. Make a `GET` request to `/api/v1/users/me/favorites?limit=10&offset=10`. | The API returns the second page of favorites, containing 10 items. |

## 2. Authentication and Authorization

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-FAV-009 | Unauthenticated Access | Verify that unauthenticated users cannot access any favorites endpoints. | 1. Without an authentication token, attempt to make a `POST`, `DELETE`, or `GET` request to any of the `/api/v1/users/me/favorites` endpoints. | The API returns a 401 Unauthorized error. |
| TC-FAV-010 | Access Other User's Favorites | Verify a user cannot list or modify another user's favorites. | 1. Authenticate as User A. <br> 2. Attempt to access an endpoint that would modify User B's favorites (this test assumes an endpoint like `/api/v1/users/{userId}/favorites` does not exist or is protected). | As the API is designed with `/me/`, direct access to other users' favorites is not possible. This test case confirms the design prevents such actions. The expected result is that there is no API to even attempt this. |

## 3. Non-Functional Test Cases

| Test Case ID | Category | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-FAV-NFR-001 | Auditing | Verify that save and unsave actions are logged. | 1. Save a destination. <br> 2. Unsave the same destination. <br> 3. Inspect the `activity_log` table or equivalent logging system. | Two new log entries are created: one for the `saved` action and one for the `unsaved` action, with correct user, destination, and timestamp details. |
| TC-FAV-NFR-002 | Performance | Verify that counting favorites is performant. | 1. Add a large number of favorites for a single destination. <br> 2. Use a performance testing tool to measure the response time of `GET /api/v1/destinations/{destinationId}/favorites/count`. | The response time remains within an acceptable SLA, indicating efficient querying. |
