# Test Cases for Admin Destination Management

## 1. Draft Management

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-001 | Create Draft | Verify an admin can create a new destination draft. | 1. As an admin, make a `POST` request to `/api/v1/admin/destination-changes` with a valid draft payload. | The system returns a 200 OK with a "Draft saved" message and the change request object. A new record is created in the `destination_change_request` table with `status='draft'`. |
| TC-ADM-002 | Update Draft | Verify an admin can update an existing draft. | 1. Create a draft. <br> 2. Make a `PUT` request to `/api/v1/admin/destination-changes/{id}` with updated fields. | The system returns a 200 OK with a "Draft saved" message. The corresponding record in `destination_change_request` is updated, and `draft_version` is incremented. |
| TC-ADM-003 | Save Incomplete Draft | Verify the system can save a draft with missing non-required fields. | 1. Make a `POST` request to create a draft with only the required fields (e.g., `name`). | The draft is saved successfully with `status='draft'`. |
| TC-ADM-004 | Stale Draft Update | Verify that the system prevents lost updates using draft versions. | 1. Open a draft for editing in two separate sessions (retrieving the same `draft_version`). <br> 2. Save a change from the first session. <br> 3. Attempt to save a change from the second session with the original (stale) `draft_version`. | The second save attempt is rejected with a 409 Conflict error. |

## 2. Submission and Approval Workflow

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-005 | Submit for Review | Verify an admin can submit a draft for review. | 1. Create a complete and valid draft. <br> 2. Make a `POST` request to `/api/v1/admin/destination-changes/{id}/submit`. | The system returns a 200 OK with a "Destination submitted for review" message. The change request status is updated to `pending_review`. |
| TC-ADM-006 | Approve Change | Verify a reviewer can approve a change, publishing it. | 1. An author submits a change for review. <br> 2. A different admin (reviewer) makes a `POST` request to `/api/v1/admin/destination-changes/{id}/approve`. | The system returns a 200 OK with an "updated successfully" message. The change is applied to the `travel_destination` table, `version` is incremented, a `destination_version` snapshot is created, and the change request status is set to `approved`. |
| TC-ADM-007 | Reject Change | Verify a reviewer can reject a change. | 1. An author submits a change for review. <br> 2. A reviewer makes a `POST` request to `/api/v1/admin/destination-changes/{id}/reject` with a `rejection_reason`. | The system returns a 200 OK with a "change rejected" message. The change request status is updated to `rejected`. The draft remains editable by the author. |
| TC-ADM-008 | Self-Approval Restriction | Verify an admin cannot approve their own submission. | 1. An admin submits a change for review. <br> 2. The same admin attempts to approve it. | The request is rejected with a 403 Forbidden error. |
| TC-ADM-009 | Edit Locked Draft | Verify that a draft in `pending_review` status cannot be edited. | 1. Submit a draft for review. <br> 2. Attempt to `PUT` an update to the change request. | The request is rejected with a 409 Conflict or 422 Invalid State error. |
| TC-ADM-010 | Resubmit After Rejection | Verify an author can edit and resubmit a rejected change. | 1. A change is rejected by a reviewer. <br> 2. The author updates the draft. <br> 3. The author resubmits the change for review. | The change request status successfully transitions back to `pending_review`. |

## 3. Delete Flow

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-011 | Request Deletion | Verify an admin can request to delete a destination. | 1. Make a `POST` request to `/api/v1/admin/destination-changes` with `action: 'delete'`. | A new change request is created with `status='pending_review'` and `action='delete'`. |
| TC-ADM-012 | Approve Deletion | Verify a reviewer can approve a deletion request. | 1. An author submits a delete request. <br> 2. A reviewer approves the request. | The destination is soft-deleted (status set to `archived`) in the `travel_destination` table. A version snapshot is created. The change request is marked `approved`. |

## 4. Public View

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-013 | View Published Content | Verify that public endpoints only return published destinations. | 1. Create a draft destination. <br> 2. Submit and approve it to publish. <br> 3. Create another draft but do not publish it. <br> 4. Make a `GET` request to `/api/v1/destinations`. | The API response includes the published destination but not the unpublished draft. |
| TC-ADM-014 | View After Update | Verify that an update to a destination is visible after approval. | 1. Publish a destination. <br> 2. Create and approve an update for it. <br> 3. Make a `GET` request to `/api/v1/destinations/{id}`. | The API returns the updated content with the new version number. |

## 5. Validation

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-015 | Required Fields | Verify that required fields are enforced on save. | 1. Attempt to save a draft with a missing required field (e.g., `name`). | The request is rejected with a 400 Bad Request error and a message indicating the missing field. |
| TC-ADM-016 | Media Validation | Verify that media uploads are validated for type and size. | 1. Attempt to upload a hero image that exceeds the `DESTINATION_IMAGE_MAX_BYTES` limit. | The upload is rejected with an appropriate error (e.g., 413 Payload Too Large). |
| TC-ADM-017 | Contact Validation | Verify that the contact payload is validated. | 1. Attempt to save a draft with an empty `contact` object. | The request is rejected with a 400 Bad Request error. |
| TC-ADM-018 | Time Validation | Verify that `closing_time` is after `opening_time`. | 1. Attempt to save a draft where `closing_time` is before `opening_time`. | The request is rejected with a 400 Bad Request error. |

## 6. Security and Permissions

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-019 | Non-Admin Access | Verify that non-admin users cannot access admin endpoints. | 1. Authenticate as a regular user. <br> 2. Attempt to make a `POST` request to `/api/v1/admin/destination-changes`. | The request is rejected with a 403 Forbidden error. |
| TC-ADM-020 | Cross-Author Editing | Verify an admin cannot edit a draft created by another admin. | 1. Admin A creates a draft. <br> 2. Admin B attempts to `PUT` an update to that draft. | The request is rejected with a 403 Forbidden error (unless business rules allow this, which should be clarified). |

## 7. Non-Functional

| Test Case ID | Category | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-ADM-NFR-001 | Auditability | Verify that audit trails are created for all actions. | 1. Perform a full workflow: create draft, submit, reject, resubmit, approve. <br> 2. Inspect the `destination_change_request` and `destination_version` tables. | All state transitions, actor IDs (`submitted_by`, `reviewed_by`), and timestamps are correctly recorded. A version snapshot is created upon approval. |
| TC-ADM-NFR-002 | Performance | Verify that the approval process is performant. | 1. Use a performance testing tool to simulate concurrent approvals. | The average response time for approvals remains within the defined SLA (e.g., <= 750 ms p95). The database transaction prevents race conditions. |
