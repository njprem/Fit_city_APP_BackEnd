# Destination Bulk Import

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-DBI-01 | CSV Upload | The system shall accept a UTF-8 CSV file containing destination metadata. | High |
| FR-DBI-02 | Row Validation | The system shall validate each row with the same business rules enforced during manual draft creation. | High |
| FR-DBI-03 | Change Request Creation | The system shall create a change request per valid row, and immediately mark it `pending_review`. | High |
| FR-DBI-04 | Error Feedback | The system shall provide feedback for every failed row. | High |
| FR-DBI-05 | Create Operation Only | The system shall focus solely on create operations; every row describes a new destination. | High |
| FR-DBI-06 | Flattened Gallery | The system shall support flattened gallery media into single-row columns. | High |
| FR-DBI-07 | No Automatic Publishing | The system shall not support automatic approval or publishing of imported destinations. | High |
| FR-DBI-08 | No Binary Uploads | The system shall not support binary or media uploads through CSV; all media must already exist at a reachable URL. | High |
| FR-DBI-09 | Required Fields | The system shall enforce required fields: `name`, `category`, `city`, `country`, `description`, `latitude`, `longitude`, `contact`, `hero_image_url`. | High |
| FR-DBI-10 | Field Content Validation | The system shall validate coordinates, contact information, hero image URL, gallery URLs, opening/closing times, and slug uniqueness. | High |

---

# Destination View Stats

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-DVS-01 | Count Destination Views | The system shall count destination detail views over sliding time windows (e.g., last 1 h/24 h/7 d) with both total hits and unique viewers. | High |
| FR-DVS-02 | Stats API | The system shall provide an API (initially admin-only) to fetch per-destination stats and “top viewed” leaderboards for dashboards. | High |
| FR-DVS-03 | Lightweight Implementation | The system shall keep implementation lightweight by querying Elasticsearch directly or via a scheduled aggregation job. | High |
| FR-DVS-04 | Data Minimization | The system shall ensure the solution keeps personal data minimized (counts only, no raw IP leakage outside Elasticsearch). | High |

---

# Elasticsearch Stack Connectivity

## Functional Requirements

| ID | Requirement Name | Description | Priority |
| --- | --- | --- | --- |
| FR-ELK-01 | Elasticsearch API Endpoint | The system shall expose the Elasticsearch REST API on TCP port 9200 for index management, querying, and cluster health checks. | High |
| FR-ELK-02 | Logstash JSON Input | The system shall expose a Logstash input on TCP port 5000 to accept newline-delimited JSON events. | High |
| FR-ELK-03 | Logstash Beats Input | The system shall expose a Logstash input on TCP port 5044 for the Beats protocol. | High |
| FR-ELK-04 | Kibana UI Endpoint | The system shall expose the Kibana UI on TCP port 5601 for browser access. | High |
| FR-ELK-05 | Direct JSON Logging | The system shall support direct JSON TCP shipping from applications to Logstash for logging. | High |
| FR-ELK-06 | Beats-based Logging | The system shall support Beats-based workflows for application logging. | High |