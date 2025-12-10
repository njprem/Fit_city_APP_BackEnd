# Test Cases for Destination Listing Filters

## 1. Functional Test Cases

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-DSF-001 | Full-text Search | Verify that full-text search works across relevant fields. | 1. Make a `GET` request to `/api/v1/destinations?query={keyword}` where `{keyword}` exists in a destination's name, city, country, category, or description. | The API returns a list of destinations that match the keyword. |
| TC-DSF-002 | Category Filtering (Single) | Verify filtering by a single category. | 1. Make a `GET` request to `/api/v1/destinations?category={category_name}`. | The API returns only destinations belonging to the specified category. |
| TC-DSF-003 | Category Filtering (Multiple) | Verify filtering by multiple categories. | 1. Make a `GET` request to `/api/v1/destinations?category={cat1}&category={cat2}`. | The API returns destinations belonging to either of the specified categories. |
| TC-DSF-004 | Rating Filtering (Min) | Verify filtering by a minimum rating. | 1. Make a `GET` request to `/api/v1/destinations?min_rating=4`. | The API returns only destinations with an average rating of 4 or higher. |
| TC-DSF-005 | Rating Filtering (Max) | Verify filtering by a maximum rating. | 1. Make a `GET` request to `/api/v1/destinations?max_rating=3`. | The API returns only destinations with an average rating of 3 or lower. |
| TC-DSF-006 | Rating Filtering (Range) | Verify filtering by a rating range. | 1. Make a `GET` request to `/api/v1/destinations?min_rating=3&max_rating=4`. | The API returns only destinations with an average rating between 3 and 4 (inclusive). |
| TC-DSF-007 | Sorting (Rating Desc) | Verify sorting by rating in descending order. | 1. Make a `GET` request to `/api/v1/destinations?sort=rating_desc`. | The API returns destinations sorted from the highest rating to the lowest. |
| TC-DSF-008 | Sorting (Alphabetical Asc) | Verify sorting alphabetically. | 1. Make a `GET` request to `/api/v1/destinations?sort=alpha_asc`. | The API returns destinations sorted by name in alphabetical order (A-Z). |
| TC-DSF-009 | Pagination | Verify that pagination works correctly. | 1. Make a `GET` request to `/api/v1/destinations?limit=5&offset=5`. | The API returns the second page of results, containing 5 destinations. |
| TC-DSF-010 | Combined Filters | Verify that multiple filters can be used together. | 1. Make a `GET` request to `/api/v1/destinations?query=park&category=Nature&min_rating=4&sort=rating_desc`. | The API returns destinations that match the keyword "park", are in the "Nature" category, have a rating of 4+, and are sorted by rating. |
| TC-DSF-011 | No Results | Verify that the API returns an empty list when no destinations match the criteria. | 1. Make a `GET` request with filters that match no destinations (e.g., `query=nonexistentkeyword`). | The API returns a 200 OK with an empty array `[]` in the response body. |

## 2. Error Handling Test Cases

| Test Case ID | Feature | Description | Steps | Expected Result |
|---|---|---|---|---|
| TC-DSF-012 | Invalid Sort Parameter | Verify the API handles invalid sort values. | 1. Make a `GET` request to `/api/v1/destinations?sort=invalid_sort_key`. | The API returns a 400 Bad Request error. |
| TC-DSF-013 | Invalid Rating Value | Verify the API handles non-numeric or out-of-range rating values. | 1. Make a `GET` request to `/api/v1/destinations?min_rating=abc`. <br> 2. Make another request with `min_rating=6`. | Both requests are rejected with a 400 Bad Request error. |
| TC-DSF-014 | Invalid Pagination | Verify the API handles invalid pagination values. | 1. Make a `GET` request to `/api/v1/destinations?limit=-1`. | The request is rejected with a 400 Bad Request error. |
