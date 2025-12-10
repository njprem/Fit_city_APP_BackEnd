# Test Cases for Reviews & Ratings

## 1. Create Review

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-RAR-001 | Create Review (Rating Only) | Verify a user can post a review with only a star rating. | 1. As an authenticated user, make a `POST` request to `/api/v1/destinations/{id}/reviews` with a `rating` value. | The API returns a 201 Created status with the new review object. The `review` table is updated. |
| TC-RAR-002 | Create Review (With Content) | Verify a user can post a review with a rating, title, and content. | 1. As an authenticated user, make a `POST` request with `rating`, `title`, and `content`. | The API returns a 201 Created status. The review is saved with all fields populated. |
| TC-RAR-003 | Create Review (With Images) | Verify a user can post a review with images. | 1. As an authenticated user, make a `POST` request with `rating` and `images[]` (multipart/form-data). | The API returns a 201 Created status. The `review` and `review_media` tables are updated, and images are uploaded to object storage. |
| TC-RAR-004 | Create Review (Duplicate) | Verify a user cannot post more than one active review for the same destination. | 1. Post a review for a destination. <br> 2. Attempt to post a second review for the same destination. | The API returns a 409 Conflict error. |
| TC-RAR-005 | Create Review (Content Without Title) | Verify that content cannot be submitted without a title. | 1. Make a `POST` request with `rating` and `content`, but no `title`. | The API returns a 400 Bad Request error. |
| TC-RAR-006 | Create Review (Unauthenticated) | Verify unauthenticated users cannot post reviews. | 1. Without authentication, attempt to `POST` a review. | The API returns a 401 Unauthorized error. |

## 2. List Reviews

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-RAR-007 | List Reviews (Public) | Verify that anyone can list reviews for a destination. | 1. As an unauthenticated user, make a `GET` request to `/api/v1/destinations/{id}/reviews`. | The API returns a 200 OK with a list of reviews and aggregate metrics. Soft-deleted reviews are not included. |
| TC-RAR-008 | List Reviews (Filtering) | Verify that filtering by rating works correctly. | 1. Make a `GET` request with `?min_rating=4`. | The API returns only reviews with a rating of 4 or 5. |
| TC-RAR-009 | List Reviews (Sorting) | Verify that sorting by creation date works. | 1. Make a `GET` request with `?sort=created_at&order=asc`. | The API returns reviews sorted from oldest to newest. |
| TC-RAR-010 | List Reviews (Pagination) | Verify pagination of the review list. | 1. For a destination with 30+ reviews, make a `GET` request with `?limit=10&offset=10`. | The API returns the second page of 10 reviews. |
| TC-RAR-011 | Aggregate Metrics | Verify the accuracy of aggregate metrics. | 1. Post several reviews with different ratings. <br> 2. Make a `GET` request to list reviews. | The `average_rating`, `total_reviews`, and `rating_counts` in the response accurately reflect the submitted reviews. |

## 3. Delete Review

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-RAR-012 | Delete Review (Owner) | Verify a user can delete their own review. | 1. Post a review. <br> 2. As the same user, make a `DELETE` request to `/api/v1/reviews/{review_id}`. | The API returns a 200 OK. The review is soft-deleted (`deleted_at` is set) and no longer appears in public listings. |
| TC-RAR-013 | Delete Review (Admin) | Verify an admin can delete any review. | 1. User A posts a review. <br> 2. An admin user makes a `DELETE` request to `/api/v1/reviews/{review_id}`. | The API returns a 200 OK. The review is soft-deleted. |
| TC-RAR-014 | Delete Review (Permission Denied) | Verify a user cannot delete another user's review. | 1. User A posts a review. <br> 2. User B attempts to `DELETE` User A's review. | The API returns a 403 Forbidden error. |
| TC-RAR-015 | Delete Already Deleted Review | Verify that deleting a soft-deleted review fails. | 1. Delete a review. <br> 2. Attempt to delete the same review again. | The API returns a 404 Not Found error. |

## 4. Validation

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-RAR-016 | Invalid Rating | Verify the system rejects out-of-range ratings. | 1. Attempt to post a review with `rating=6`. | The API returns a 400 Bad Request error. |
| TC-RAR-017 | Invalid Image Type | Verify the system rejects disallowed image MIME types. | 1. Attempt to post a review with a non-image file (e.g., a `.txt` file). | The API returns a 400 Bad Request error. |
| TC-RAR-018 | Image Count Limit | Verify the system enforces the maximum number of images per review. | 1. Attempt to post a review with more than the allowed number of images (e.g., 6 images if the limit is 5). | The API returns a 400 Bad Request error. |
| TC-RAR-019 | Non-existent Destination | Verify reviews cannot be posted for a destination that doesn't exist. | 1. Attempt to `POST` a review to `/api/v1/destinations/{non_existent_id}/reviews`. | The API returns a 404 Not Found error. |
