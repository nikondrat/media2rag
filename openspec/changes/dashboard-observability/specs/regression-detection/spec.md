## ADDED Requirements

### Requirement: Regression comparison
The system SHALL compare current-period pipeline metrics against a baseline period to detect regressions.

#### Scenario: Default comparison
- **WHEN** `GET /api/debug/regressions?period=24h` is called with no baseline
- **THEN** the baseline is set to the 24 hours before the current period start

#### Scenario: Custom comparison
- **WHEN** `GET /api/debug/regressions?period=24h&baseline=48h&baseline_duration=24h` is called
- **THEN** the current period is the last 24h and the baseline is 48h-24h ago

### Requirement: Seven regression signals
The system SHALL track seven signals: avg_score, pass_rate, avg_latency, avg_tokens, error_rate, embedding_quality, and negative_feedback_ratio.

#### Scenario: All signals computed
- **WHEN** regression analysis runs
- **THEN** all seven signals are computed for both current and baseline periods
- **AND** each signal has current value, baseline value, delta, delta percent, status, and direction

### Requirement: Regression alert thresholds
The system SHALL classify signals as "major" (🔴), "minor" (🟡), or "ok" (✅) based on threshold violations.

#### Scenario: Score drop threshold
- **WHEN** avg_score current < baseline - 0.1
- **THEN** status is "major"
- **WHEN** avg_score current < baseline - 0.05
- **THEN** status is "minor"

#### Scenario: Pass rate drop threshold
- **WHEN** pass_rate current < baseline - 5%
- **THEN** status is "minor"
- **WHEN** pass_rate current < baseline - 10%
- **THEN** status is "major"

#### Scenario: Latency and error thresholds
- **WHEN** avg_latency current > baseline * 1.5
- **THEN** status is "minor"
- **WHEN** error_rate current > baseline + 2%
- **THEN** status is "major"
- **WHEN** error_rate current > baseline + 1%
- **THEN** status is "minor"

### Requirement: Top regressed pipeline runs
The system SHALL identify individual pipeline runs with the largest score drops compared to similar runs in the baseline.

#### Scenario: Top regressions returned
- **WHEN** regression analysis runs
- **THEN** up to 10 runs with the lowest scores in the current period are returned with their scores and baseline comparison
