# Admin Destination Management

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-ADM-01 | Create Destination Content | The system shall allow privileged admins to compose new destination content (metadata, hero imagery, galleries, contact info, operating hours, geo coordinates). | High |
| FR-ADM-02 | Draft and Review State | The system shall enable draft and “submit for review” behavior so authors can stage work while reviewers control publication. | High |
| FR-ADM-03 | Approval for Publication | The system shall ensure no single admin can unilaterally alter the live destination list—an approval step is required to publish or delete content. | High |
| FR-ADM-04 | Versioned Snapshots | The system shall persist versioned snapshots of published destinations (`version` numbers + change metadata) to enable rollback and auditability. | High |
| FR-ADM-05 | Read APIs | The system shall provide read APIs that return stable published data to end users while exposing administrative views for pending requests. | High |
| FR-ADM-06 | Confirmation Payload | The system shall emit a confirmation payload (e.g., `{ "message": "Destination updated successfully" }`) when approvals complete and changes are applied. | High |
| FR-ADM-07 | Audit Trails | The system shall maintain structured audit trails and metrics covering submissions, approvals, rejections, and publication outcomes. | High |
| FR-ADM-08 | Field Validation | The system shall validate required fields (name, status, coordinates, contact channels as configured) **before** saving drafts and submissions. | High |
| FR-ADM-09 | Media Validation | The system shall enforce hero image and gallery media MIME types/size limits; store sanitized metadata. | High |
| FR-ADM-10 | Contact Validation | The system shall ensure contact payload contains at least one reachable channel (phone, email, or website) and normalize formats. | High |
| FR-ADM-11 | Time Validation | The system shall validate `opening_time`/`closing_time` (closing must be after opening within the configured timezone; overnight support via flag). | High |
| FR-ADM-12 | Status Transitions | The system shall maintain `status` transitions: drafts -> pending_review -> approved/rejected; only approved requests alter live table. | High |
| FR-ADM-13 | Self-approval Restriction | The system shall prevent a submitter from approving their own change; approval requires a different admin account. | High |
| FR-ADM-14 | Rejection Reason | The system shall require a rejection reason (min length 10 characters) to guide author revisions. | Medium |

---

# Destination Listing Filters

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-DSF-01 | Full-text Search | The system shall provide a full-text search across name, city, country, category, and description. | High |
| FR-DSF-02 | Category Filtering | The system shall allow filtering results by one or more categories. | High |
| FR-DSF-03 | Rating Filtering | The system shall allow filtering results by a minimum and maximum average review rating. | High |
| FR-DSF-04 | Sorting | The system shall support sorting results by rating (ascending/descending), alphabetical order (ascending/descending), and last updated time (descending). | High |
| FR-DSF-05 | Pagination | The system shall support pagination of results using limit and offset. | High |

---

# Favorites API

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-FAV-01 | Save a Destination | The system shall allow an authenticated user to save a destination as a favorite. | High |
| FR-FAV-02 | Unsave a Destination | The system shall allow an authenticated user to remove a destination from their favorites. | High |
| FR-FAV-03 | List Favorites | The system shall provide a way for an authenticated user to list their saved destinations. | High |
| FR-FAV-04 | Count Favorites | The system shall provide a way to count the number of users who have favorited a specific destination. | Medium |
| FR-FAV-05 | Prevent Duplicates | The system shall prevent a user from saving the same destination as a favorite multiple times. | High |
| FR-FAV-06 | Activity Logging | The system shall log all save and unsave actions for auditing and analytics. | Medium |

---

# Reviews & Ratings Feature

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-RAR-01 | Publish Review | The system shall enable signed-in users to publish one review per destination. | High |
| FR-RAR-02 | Review Content | A review shall contain a numeric rating (0–5), optional title, optional content, and up to N supporting images. | High |
| FR-RAR-03 | List Reviews API | The system shall expose a public read API that lists reviews for a destination with pagination, rating/time filters, title/content, reviewer display name, and uploaded media URLs. | High |
| FR-RAR-04 | Aggregate Metrics | The public read API shall return aggregate metrics such as average rating, total review count, and per-rating distribution. | High |
| FR-RAR-05 | Delete Review | The system shall allow review owners and admins to delete a review, including the associated media objects and metadata. | High |