## ADDED Requirements

### Requirement: Feedback submission
The system SHALL allow users to submit feedback on pipeline runs.

#### Scenario: Submit feedback with rating
- **WHEN** a user submits `POST /api/debug/feedback` with `{run_id, rating: 4}`
- **THEN** a `feedback` record is created with the provided data and a generated id

#### Scenario: Submit feedback with like/dislike
- **WHEN** a user submits feedback with `{run_id, like_dislike: "dislike"}`
- **THEN** the `like_dislike` field is stored with the value

#### Scenario: Submit feedback with category and comment
- **WHEN** a user submits `{run_id, category: "hallucination", comment: "The answer contains false information"}`
- **THEN** both `category` and `comment` fields are stored

### Requirement: Feedback fields
The system SHALL support the following feedback fields: rating (1-5), like_dislike ("like"/"dislike"/""), category ("accurate"/"hallucination"/"irrelevant"/"incomplete"/"other"), and comment (free text).

#### Scenario: Valid categories
- **WHEN** feedback is submitted with category "accurate"
- **THEN** it is stored as-is
- **WHEN** feedback is submitted with an invalid category
- **THEN** the category is stored as "other"

### Requirement: Feedback impact on pipeline score
The system SHALL adjust pipeline score and metrics based on negative feedback.

#### Scenario: Dislike penalizes score
- **WHEN** feedback with `like_dislike: "dislike"` is submitted for a run
- **THEN** the run's effective score is penalized by 10% (multiplied by 0.9)
- **AND** the pass rate recalculation includes this penalty

#### Scenario: Feedback flagged for regression detection
- **WHEN** a pipeline run has negative feedback (rating < 3 or like_dislike = "dislike")
- **THEN** the run is flagged in the regression analysis as a negative signal

### Requirement: Feedback listing
The system SHALL provide an API for listing feedback with summary statistics.

#### Scenario: List feedback
- **WHEN** `GET /api/debug/feedback?period=7d` is called
- **THEN** the response includes total count, likes/dislikes, avg rating, category distribution, and the feedback entries

#### Scenario: Feedback SSE event
- **WHEN** feedback is submitted
- **THEN** an SSE `feedback` event is broadcast with run_id and rating
