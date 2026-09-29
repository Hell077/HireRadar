# HireRadar — End-to-End Implementation Plan

## 0. Purpose

Implement the complete HireRadar flow end to end:

```text
discover sources
→ collect vacancies
→ normalize and deduplicate jobs
→ match jobs against candidate profile/resume
→ rank matches
→ notify user in Telegram
→ user presses Apply
→ create application
→ automatically submit when supported
→ request missing input when required
→ report final result back to Telegram
```

The implementation must remain deterministic, observable, idempotent, and safe to retry.

Do not redesign the entire project. Extend the existing architecture.

---

# Block 1 — Final Target Architecture

## Goal

The final backend flow should look like this:

```text
┌───────────────────────┐
│ Source Discovery      │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ Source Registry       │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ Source Workers        │
│                       │
│ Greenhouse            │
│ Lever                 │
│ Ashby                 │
│ GitHub                │
│ RemoteOK              │
│ Jobicy                │
│ WeWorkRemotely        │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ raw_jobs              │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ Normalize + Dedup     │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ jobs                  │
└──────────┬────────────┘
           │
           ▼
     job.created
     job.updated
     job.closed
           │
           ▼
┌───────────────────────┐
│ Matching Worker       │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ user_job_matches      │
└──────────┬────────────┘
           │
           ▼
     match.created
           │
           ▼
┌───────────────────────┐
│ Notification Worker   │
└──────────┬────────────┘
           │
           ▼
┌───────────────────────┐
│ Telegram              │
└──────────┬────────────┘
           │
     Apply button
           │
           ▼
┌───────────────────────┐
│ Job Application       │
│ Service               │
└──────────┬────────────┘
           │
           ▼
 application.requested
           │
           ▼
┌───────────────────────┐
│ Application Worker    │
└──────────┬────────────┘
           │
     ┌─────┴───────────────┐
     │                     │
     ▼                     ▼
ATS-native adapter    Browser adapter
     │                     │
     └──────────┬──────────┘
                │
                ▼
        submitted
        needs_input
        manual_required
        failed
                │
                ▼
             Telegram
```

---

# Block 2 — Global Engineering Rules

All new code must follow these rules.

## 2.1 Idempotency

Every asynchronous operation must be safe to execute multiple times.

Required unique constraints:

```text
one match per user + job
one notification per user + job + type
one application per user + job
one active application attempt per application
```

Never depend on "this handler should only run once".

---

## 2.2 No long database transactions

Never perform the following while holding a PostgreSQL transaction open:

```text
HTTP requests
Telegram API calls
ATS API calls
browser automation
PDF parsing
LLM calls
long matching loops
```

Transactions are only for:

```text
claim work
persist state
ack work
schedule retry
```

---

## 2.3 Claim/lease model

Background work must use leases instead of holding row locks during processing.

Generic flow:

```text
BEGIN

claim one item
set:
    status = processing
    locked_by
    locked_until

COMMIT

perform work outside transaction

BEGIN

persist result
mark processed

COMMIT
```

Expired leases must be reclaimable.

---

## 2.4 Retry policy

Transient failures:

```text
network timeout
HTTP 429
HTTP 5xx
temporary PostgreSQL problem
temporary Telegram failure
temporary browser failure
```

must retry with exponential backoff.

Permanent failures:

```text
invalid job
unsupported provider
invalid application form
required manual authentication
CAPTCHA
deleted vacancy
```

must not retry forever.

---

## 2.5 Observability

Every worker must expose:

```text
processed total
success total
failed total
retry total
processing latency
queue depth
oldest pending item age
```

Every important log should include identifiers when applicable:

```text
service
event_id
source_id
job_id
user_id
application_id
attempt_id
provider
```

Never log:

```text
passwords
API tokens
resume content
private form answers
authorization headers
```

---

# Block 3 — Refactor Existing Worker Processing

## Goal

Remove long PostgreSQL transactions from matching and notification processing.

This block must be completed before implementing auto-apply.

---

## 3.1 Extend outbox events

Add fields:

```sql
status text NOT NULL DEFAULT 'pending'
locked_by text
locked_until timestamptz
last_error text
failed_at timestamptz
processed_at timestamptz
attempts integer NOT NULL DEFAULT 0
available_at timestamptz NOT NULL DEFAULT now()
```

Allowed states:

```text
pending
processing
processed
failed
```

Create indexes for:

```text
pending + available_at
processing + locked_until
failed
```

---

## 3.2 Implement reusable outbox claim logic

Create a small internal abstraction.

Suggested package:

```text
internal/outbox
```

Suggested types:

```go
type Event struct {
    ID            string
    EventType     string
    AggregateType string
    AggregateID   string
    Payload       json.RawMessage

    Attempts      int
    AvailableAt   time.Time

    LockedBy      *string
    LockedUntil   *time.Time
}

type Store interface {
    Claim(
        ctx context.Context,
        workerID string,
        eventTypes []string,
        lease time.Duration,
    ) (*Event, error)

    Complete(
        ctx context.Context,
        eventID string,
        workerID string,
    ) error

    Retry(
        ctx context.Context,
        eventID string,
        workerID string,
        cause error,
    ) error

    Fail(
        ctx context.Context,
        eventID string,
        workerID string,
        cause error,
    ) error
}
```

The worker must not keep the claim transaction open during processing.

---

## 3.3 Refactor matching worker

Current problem:

```text
claim outbox row
→ keep transaction open
→ recalculate matches
→ acknowledge event
```

Target:

```text
claim event
→ commit
→ recalculate matches
→ complete event
```

On crash:

```text
locked_until expires
→ another worker can reclaim event
```

---

## 3.4 Refactor notification worker

Current problem:

```text
SELECT notification FOR UPDATE
→ call Telegram while transaction is open
→ mark sent
```

Target:

```text
claim notification
→ status = delivering
→ commit

call Telegram

success:
    mark sent

temporary failure:
    status = pending
    scheduled_at = retry time

permanent failure / retry limit:
    status = failed
```

Add:

```text
locked_by
locked_until
```

to notifications if required.

---

## Acceptance Criteria

- Telegram HTTP calls happen outside SQL transactions.
- Matching calculation happens outside outbox claim transactions.
- Killing a worker after claiming work does not permanently lose the task.
- Expired work can be reclaimed.
- Existing integration tests still pass.
- Add integration test for worker crash after claim.

---

# Block 4 — Fix Matching Correctness Before Scaling

## Goal

Remove the current correctness problem caused by limited candidate selection followed by full stale-match deletion.

---

## 4.1 Remove dangerous full replacement behavior

Current problematic behavior:

```text
CandidateJobs LIMIT 5000
→ calculate matches
→ delete every previous match not included in returned IDs
```

If more than 5000 jobs qualify, valid old matches may be removed.

Do not use this approach.

---

## 4.2 Introduce versioned matching

Add profile matching version:

```text
user_profiles.match_version bigint
```

Increment it whenever matching-relevant candidate data changes:

```text
country
seniority
positions
skills
resume parsed data
job preferences
matching weights
```

Add job version:

```text
jobs.match_version bigint
```

Increment it when matching-relevant job data changes:

```text
title
description
skills
salary
location
countries
remote policy
employment type
status
```

Extend:

```text
user_job_matches
```

with:

```text
candidate_version bigint
job_version bigint
```

A match is stale when:

```text
user_job_matches.candidate_version != user_profiles.match_version
OR
user_job_matches.job_version != jobs.match_version
```

---

## 4.3 Process profile rematches in chunks

Do not load 5000 jobs into memory and then delete everything else.

Suggested API:

```go
type JobPage struct {
    Jobs       []jobdomain.Job
    NextCursor string
}
```

Process:

```text
profile.changed
→ load candidate snapshot
→ scan candidate jobs in chunks
→ evaluate each chunk
→ upsert matches
→ delete stale matches only after complete successful scan
```

Suggested chunk size:

```text
250–1000 jobs
```

The final cleanup must be based on version, not on a single in-memory list.

Example:

```sql
DELETE FROM user_job_matches
WHERE user_id = $1
AND candidate_version < $current_version;
```

Only execute after the full refresh completed successfully.

---

## 4.4 Optimize job → candidate matching

Current model:

```text
new job
→ load every candidate
→ multiple SQL queries per candidate
```

Replace with preselection.

Create repository method:

```go
CandidateIDsForJob(
    ctx context.Context,
    job jobdomain.Job,
    cursor string,
    limit int,
) (IDs []user.UserID, nextCursor string, err error)
```

Use cheap SQL filters first:

```text
active users
country compatibility
remote policy compatibility
employment type compatibility
maximum job age
source preferences
basic job family compatibility
```

Then run exact Go scoring only against selected candidates.

---

## Acceptance Criteria

- More than 5000 matching jobs cannot cause valid matches to disappear.
- A failed full profile refresh does not delete existing valid matches.
- New-job matching does not perform 5–10 SQL queries for every registered user.
- Matching can be resumed after worker failure.

---

# Block 5 — Improve Job Classification

## Goal

Replace growing hardcoded negative-role rules with structured job classification.

---

## 5.1 Add job family

Add:

```text
jobs.job_family
```

Initial values:

```text
software_engineering
data
devops
security
qa
product
design
sales
marketing
support
hr
finance
operations
management
other
unknown
```

---

## 5.2 Add position normalization

Normalize common titles:

```text
Backend Engineer
Backend Developer
Software Engineer, Backend
Golang Backend Engineer
Go Engineer
```

into structured data:

```go
type PositionClassification struct {
    Family      string
    Speciality  string
    Seniority   string
}
```

Example:

```text
family      = software_engineering
speciality  = backend
seniority   = senior
```

---

## 5.3 Store classification confidence

Add:

```text
job_family_confidence
seniority_confidence
location_confidence
salary_confidence
```

Do not treat inferred and provider-supplied data as equally reliable.

---

# Block 6 — Improve Matching Model

## Goal

Keep deterministic matching as the primary ranking mechanism.

Do not introduce LLM ranking as the main source of truth.

---

## 6.1 Hard filters

Hard reject only when there is strong evidence.

Examples:

```text
job closed
candidate country explicitly prohibited
candidate requested remote only but job is explicitly onsite
employment type explicitly disallowed
salary definitely below hard minimum
job age exceeds configured hard maximum
job family definitely unrelated
required language definitely missing
```

Unknown data must usually remain unknown instead of becoming rejection.

---

## 6.2 Structured scoring

Recommended initial components:

```text
skills             35
position            20
seniority           15
location            10
salary               8
experience           7
employment type      5
-----------------------
total               100
```

Weights remain user-configurable if current product behavior requires it.

---

## 6.3 Seniority distance

Replace exact-only seniority scoring.

Example mapping:

```text
same level                      100
candidate one level above        80
candidate one level below        60
candidate two levels away        25
large mismatch                    0
unknown                          50
```

Keep values centralized and unit-tested.

---

## 6.4 Skill matching

Skill score must support:

```text
required skills
preferred skills
candidate years
candidate skill level
job skill confidence
candidate skill confidence
aliases
```

Example output:

```json
{
  "code": "skills",
  "score": 84,
  "matched": [
    "Go",
    "PostgreSQL",
    "Docker"
  ],
  "missing_required": [],
  "missing_preferred": [
    "AWS"
  ]
}
```

---

## 6.5 Add match confidence

Extend result:

```go
type Result struct {
    JobID      string
    Score      int
    Confidence int
    Eligible   bool
    Components []Component
    Exclusions []string
}
```

Difference:

```text
score = how well the known evidence matches
confidence = how complete/reliable the evidence is
```

Unknown salary must reduce confidence rather than pretending that salary matches at 50%.

---

## 6.6 Persist explanation

Store enough structured information to show:

```text
Why this job matched
Why this job lost points
What information was unknown
```

Do not regenerate explanation from plain score later.

---

# Block 7 — Improve Resume Parsing

## Goal

Convert a resume into structured candidate experience instead of only keyword detection.

---

## 7.1 Keep existing secure PDF extraction

Keep current limits:

```text
file size
page limit
stream limit
operator limit
glyph limit
image limit
```

Do not weaken parser protections.

---

## 7.2 Introduce structured resume model

Suggested model:

```go
type ParsedResume struct {
    ResumeID              string
    Text                  string
    Skills                []DetectedSkill
    Positions             []DetectedPosition
    Experiences           []Experience
    Languages             []Language
    TotalExperienceMonths int
}

type Experience struct {
    Company    string
    Title      string
    StartDate  *time.Time
    EndDate    *time.Time
    Current    bool
    Skills     []DetectedSkill
    Confidence float64
}
```

---

## 7.3 Estimate skill experience

Derive:

```text
first usage
last usage
estimated months
current usage
```

Example:

```text
Go:
    estimated_months = 42
    last_used = current

PostgreSQL:
    estimated_months = 48

Python:
    estimated_months = 12
    last_used = 2022
```

Never claim exact experience when only an estimate exists.

---

## 7.4 Preserve manual profile edits

Resume parsing must not blindly overwrite user-provided fields.

Use separate sources:

```text
manual
resume
inferred
```

A manually entered skill should have precedence over lower-confidence inference.

---

# Block 8 — Source Coverage System

## Goal

Choose future source adapters based on measured coverage instead of guessing.

---

## 8.1 Add provider classification to discovery candidates

Track:

```text
provider
provider_confidence
status
last_checked_at
```

Possible status:

```text
supported
unsupported
unknown
manual_review
invalid
```

---

## 8.2 Add coverage metrics

Expose:

```text
discovered companies
resolved careers pages
supported providers
unsupported providers
unknown providers
invalid pages
```

Break down unsupported providers:

```text
Workday
SmartRecruiters
Workable
Teamtailor
Recruitee
Personio
BambooHR
Comeet
Pinpoint
other
```

---

## 8.3 Prioritize connectors by coverage

Implement new job source connectors in descending real coverage order.

Do not prioritize a provider only because it is easy to implement.

---

# Block 9 — Introduce Job Application Domain

## Goal

Create a separate bounded context responsible for actual job applications.

Use:

```text
internal/jobapplication
```

Do not put auto-apply logic into:

```text
notification
matching
source
```

---

## 9.1 Package structure

Create:

```text
internal/jobapplication/
    domain/
    application/
    adapters/
        postgres/
        greenhouse/
        lever/
        ashby/
        browser/
```

Suggested structure:

```text
internal/jobapplication/domain/application.go
internal/jobapplication/domain/form.go
internal/jobapplication/domain/status.go

internal/jobapplication/application/service.go
internal/jobapplication/application/worker.go
internal/jobapplication/application/registry.go

internal/jobapplication/adapters/postgres/store.go

internal/jobapplication/adapters/greenhouse/client.go
internal/jobapplication/adapters/lever/client.go
internal/jobapplication/adapters/ashby/client.go

internal/jobapplication/adapters/browser/client.go
```

---

# Block 10 — Application Database Schema

## Goal

Persist every application and every attempt.

---

## 10.1 Applications table

Create:

```sql
CREATE TABLE job_applications (
    id uuid PRIMARY KEY,

    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,

    provider text NOT NULL,

    status text NOT NULL,

    requested_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    submitted_at timestamptz,

    attempts integer NOT NULL DEFAULT 0,

    locked_by text,
    locked_until timestamptz,

    last_error_code text,
    last_error_message text,

    form_snapshot jsonb,
    result jsonb,

    UNIQUE(user_id, job_id)
);
```

Statuses:

```text
requested
preparing
needs_input
ready
submitting
submitted
manual_required
failed
cancelled
```

---

## 10.2 Application attempts

Create:

```sql
CREATE TABLE job_application_attempts (
    id uuid PRIMARY KEY,

    application_id uuid NOT NULL
        REFERENCES job_applications(id)
        ON DELETE CASCADE,

    attempt_number integer NOT NULL,

    status text NOT NULL,

    provider text NOT NULL,

    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,

    error_code text,
    error_message text,

    result jsonb,

    UNIQUE(application_id, attempt_number)
);
```

---

## 10.3 Application questions

Create:

```sql
CREATE TABLE job_application_questions (
    id uuid PRIMARY KEY,

    application_id uuid NOT NULL
        REFERENCES job_applications(id)
        ON DELETE CASCADE,

    external_key text,

    question text NOT NULL,

    question_type text NOT NULL,

    required boolean NOT NULL DEFAULT true,

    options jsonb,

    answer jsonb,

    answer_source text,

    status text NOT NULL,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

Question statuses:

```text
unanswered
answered
skipped
invalid
```

Answer sources:

```text
profile
resume
saved_answer
user
generated
```

---

# Block 11 — Application Profile

## Goal

Store the information required to fill external application forms.

This is different from matching preferences.

---

## 11.1 Create application profile

Suggested fields:

```go
type ApplicationProfile struct {
    UserID string

    FirstName string
    LastName  string

    Email string
    Phone string

    Country string
    City    string

    LinkedInURL string
    GitHubURL   string
    WebsiteURL  string

    ResumeID string

    ExpectedSalary *Money
    NoticePeriodDays *int

    WorkAuthorization []WorkAuthorization

    CustomAnswers map[string]string
}
```

---

## 11.2 Sensitive data

Application profile may contain PII.

Requirements:

```text
never expose it in logs
never put full profile into outbox payloads
never put it into metrics labels
return only required fields through API
```

If encryption-at-rest support is added later, design repository interfaces so storage encryption can be introduced without changing the domain.

---

# Block 12 — Telegram Apply Flow

## Goal

Add the actual Apply button.

---

## 12.1 Notification buttons

Change Telegram notification buttons to:

```text
Apply
Open

Save
Hide
```

Remove the current meaning of:

```text
Applied
```

from the primary action row.

Manual "already applied elsewhere" can remain available through another command/API if needed.

---

## 12.2 Callback

New callback:

```text
job:apply:<job_id>
```

Telegram webhook must:

```text
1. validate Telegram account
2. validate job ID
3. confirm user owns the match
4. call JobApplicationService.Request()
5. return callback response immediately
```

Do not submit the application inside the webhook request.

---

## 12.3 Service API

Suggested method:

```go
func (s *Service) Request(
    ctx context.Context,
    userID user.UserID,
    jobID string,
) (domain.Application, error)
```

Behavior:

```text
existing submitted application:
    return existing application

existing active application:
    return existing application

no application:
    insert requested application
    emit application.requested
```

Operation must be idempotent.

---

## 12.4 Telegram acknowledgement

Immediately answer:

```text
Application queued
```

Do not wait for external submission.

---

# Block 13 — Application Event Flow

## Goal

Make application submission asynchronous.

---

## 13.1 New outbox events

Use:

```text
application.requested
application.input_provided
application.retry_requested
```

Do not put full resume/application profile into event payloads.

Payload should contain IDs only.

Example:

```json
{
  "application_id": "uuid"
}
```

---

## 13.2 Application worker

Flow:

```text
claim application event
→ load application
→ load job
→ load application profile
→ resolve application provider
→ prepare form
→ determine missing answers
```

Then:

```text
no missing answers
→ ready
→ submit
```

or:

```text
missing answers
→ needs_input
→ notify Telegram
```

or:

```text
CAPTCHA/login/manual step required
→ manual_required
```

or:

```text
permanent unsupported flow
→ manual_required
```

---

# Block 14 — Application Provider Interface

## Goal

Define stable contracts before implementing ATS-specific logic.

Suggested interface:

```go
type Provider interface {
    Name() string

    Supports(job jobdomain.Job) bool

    Prepare(
        ctx context.Context,
        job jobdomain.Job,
        profile domain.ApplicationProfile,
    ) (domain.PreparedForm, error)

    Submit(
        ctx context.Context,
        application domain.Application,
        form domain.PreparedForm,
    ) (domain.SubmissionResult, error)
}
```

---

## 14.1 Prepared form

Suggested structure:

```go
type PreparedForm struct {
    Fields    []Field
    Questions []Question
    Metadata  map[string]string
}

type Field struct {
    Key      string
    Type     string
    Required bool
    Value    any
}
```

---

## 14.2 Result

Suggested:

```go
type SubmissionResult struct {
    Status string

    ExternalApplicationID string
    ConfirmationURL       string

    SubmittedAt time.Time
}
```

---

# Block 15 — Provider Capability Model

## Goal

Do not assume that every source provider also supports auto-apply.

Add explicit capabilities:

```go
type ProviderCapabilities struct {
    Discovery bool
    FetchJobs bool
    AutoApply bool
}
```

Example:

```text
Greenhouse:
    discovery = true
    fetch = true
    auto_apply = true

RemoteOK:
    discovery = false
    fetch = true
    auto_apply = false
```

Expose this information through backend API if useful for UI.

---

# Block 16 — Implement First Auto-Apply Provider

## Goal

Do not implement all ATS providers at once.

Implement one complete provider end to end first.

Recommended order:

```text
1. Greenhouse
2. Lever
3. Ashby
4. browser fallback
```

The exact first provider may change if real endpoint investigation shows another provider is simpler/more stable.

---

## 16.1 Greenhouse implementation requirements

The adapter must support:

```text
discover application form
map standard fields
upload resume
map structured questions
submit application
return external result
classify errors
```

Error categories:

```text
temporary
validation
missing_input
manual_required
unsupported
job_closed
rate_limited
```

---

## 16.2 Do not infer unknown answers

The adapter must never silently invent answers to questions such as:

```text
visa sponsorship
work authorization
salary expectations
security clearance
criminal history
demographic questions
relocation
legal declarations
```

Unknown required answers must become:

```text
needs_input
```

---

# Block 17 — Missing Input Flow

## Goal

Continue an application after the user answers required questions.

---

## 17.1 Unknown question

Application worker discovers:

```text
Do you require visa sponsorship?
```

No trusted answer exists.

Persist question:

```text
status = unanswered
```

Set application:

```text
status = needs_input
```

Send Telegram message.

---

## 17.2 Telegram message

Example:

```text
Acme needs more information

Senior Go Engineer

Question:
Do you require visa sponsorship?

[Yes]
[No]
[Open application]
```

---

## 17.3 User answer

Callback:

```text
application_answer:<question_id>:<answer>
```

Flow:

```text
validate ownership
→ save answer
→ if all required questions answered
    emit application.input_provided
→ worker resumes application
```

---

## 17.4 Reusable answers

Some answers can be reusable:

```text
work authorization
visa sponsorship
willingness to relocate
notice period
```

Store reusable answers only when they are semantically stable and user-approved.

Do not automatically reuse arbitrary free-text answers for unrelated questions.

---

# Block 18 — Manual Required Flow

## Goal

Stop automation cleanly when human interaction is necessary.

Examples:

```text
CAPTCHA
OTP
login
Google authentication
Microsoft authentication
Cloudflare challenge
provider-specific anti-bot
unsupported file upload flow
legal consent that requires explicit review
```

Set:

```text
application.status = manual_required
```

Send:

```text
Manual action required

Acme · Senior Go Engineer

Reason:
CAPTCHA / authentication required

[Open application]
```

Do not build CAPTCHA bypass logic.

---

# Block 19 — Browser Automation Worker

## Goal

Support unsupported web forms without coupling browser automation directly into the Go API process.

Recommended architecture:

```text
apps/backend
    Go
    domain logic
    state machines
    persistence
    queues

apps/browser-worker
    Playwright
    browser automation only
```

---

## 19.1 Communication

Prefer a narrow internal API or queue.

Example request:

```json
{
  "application_id": "uuid"
}
```

Browser worker loads required data through authenticated internal backend endpoints or through a purpose-built job payload with minimum required fields.

Avoid passing secrets through command-line arguments.

---

## 19.2 Browser worker responsibilities

Only:

```text
open application URL
inspect form
fill known fields
upload resume
return discovered questions
submit if allowed
detect manual barriers
capture structured failure result
```

Do not move business state transitions into the browser worker.

The Go backend owns application state.

---

# Block 20 — Telegram Status Updates

## Goal

Make asynchronous application state visible to the user.

Send updates only on meaningful transitions.

---

## Required messages

### Queued

```text
Application queued
```

### Needs input

```text
More information is required
```

### Submitted

```text
Application submitted

Senior Go Engineer
Acme

Submitted at: ...
```

### Manual required

```text
Manual action required
```

### Permanent failure

```text
Application could not be submitted automatically

Reason: ...
[Open application]
```

Avoid sending internal stack traces or raw provider error bodies.

---

# Block 21 — Feedback Loop

## Goal

Use user actions to improve ranking.

Extend job feedback reasons.

Suggested values:

```text
wrong_stack
wrong_role
wrong_seniority
wrong_location
wrong_salary
wrong_company
duplicate
already_seen
not_interested
other
```

Telegram flow:

```text
Hide
→ optional reason selection
```

Store reason.

Do not immediately introduce machine learning.

First expose metrics that correlate match score with actual behavior.

---

# Block 22 — Metrics

## Source metrics

```text
hireradar_source_sync_total
hireradar_source_sync_failed_total
hireradar_jobs_fetched_total
hireradar_jobs_created_total
hireradar_jobs_updated_total
hireradar_jobs_closed_total
```

---

## Matching metrics

```text
hireradar_matching_events_total
hireradar_matches_created_total
hireradar_matches_removed_total
hireradar_matching_duration_seconds
hireradar_match_score_bucket
hireradar_match_confidence_bucket
```

---

## Notification metrics

```text
hireradar_notifications_queued_total
hireradar_notifications_sent_total
hireradar_notifications_failed_total
hireradar_notifications_retry_total
```

---

## Application metrics

```text
hireradar_applications_requested_total
hireradar_applications_prepared_total
hireradar_applications_needs_input_total
hireradar_applications_submitted_total
hireradar_applications_manual_total
hireradar_applications_failed_total
hireradar_application_duration_seconds
```

Do not put:

```text
user_id
job_id
application_id
company name
email
```

into metric labels.

---

# Block 23 — Product Quality Metrics

Track conversion:

```text
notification → open
notification → save
notification → hide
notification → apply
apply → submitted
```

Track by score bucket:

```text
70–79
80–89
90–100
```

Track by confidence bucket.

Use these metrics to determine whether matching improvements actually improve user behavior.

---

# Block 24 — API Additions

Suggested endpoints.

## Application profile

```text
GET  /api/v1/application-profile
PUT  /api/v1/application-profile
```

---

## Applications

```text
POST /api/v1/jobs/{job_id}/apply

GET /api/v1/applications
GET /api/v1/applications/{id}

POST /api/v1/applications/{id}/cancel
POST /api/v1/applications/{id}/retry
```

---

## Questions

```text
GET  /api/v1/applications/{id}/questions
POST /api/v1/application-questions/{id}/answer
```

---

## Admin

```text
GET  /api/v1/admin/outbox/failed
POST /api/v1/admin/outbox/{id}/retry

GET  /api/v1/admin/applications/failed
POST /api/v1/admin/applications/{id}/retry
```

Keep OpenAPI generation and frontend generated types updated in the same change.

---

# Block 25 — Separate API and Worker Runtime Modes

## Goal

Allow deployment to scale API and workers independently.

Do not split into microservices yet.

Use the same Go codebase with runtime mode.

Example:

```text
HIRERADAR_MODE=api
HIRERADAR_MODE=worker
HIRERADAR_MODE=all
```

Possible worker groups:

```text
source
discovery
matching
notification
resume
application
```

Later:

```text
docker compose

api x2
worker-source x1
worker-matching x2
worker-notification x1
worker-application x2
browser-worker x2
```

Keep PostgreSQL as the coordination backend for now.

Do not introduce Kafka/NATS/RabbitMQ without demonstrated need.

---

# Block 26 — CI

Create:

```text
.github/workflows/ci.yml
```

Required checks.

## Go backend

```bash
gofmt check
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

Run integration tests with PostgreSQL.

---

## Web

```bash
pnpm install --frozen-lockfile
pnpm lint
pnpm check-types
pnpm build
```

---

## Generated contracts

Generate API client:

```bash
pnpm gen:client
```

Then ensure generated output is committed and clean:

```bash
git diff --exit-code
```

---

## Security

Add:

```bash
govulncheck ./...
```

Run dependency audit where appropriate.

---

# Block 27 — End-to-End Integration Tests

## Test 1 — Job ingestion to Telegram

```text
seed source
→ source worker fetches vacancy
→ raw job saved
→ normalized job created
→ job.created emitted
→ matching worker processes
→ user_job_match created
→ match.created emitted
→ notification queued
→ fake Telegram receives message
```

Assert no duplicate notifications after retry/restart.

---

## Test 2 — Worker crash recovery

```text
claim event
→ simulate process death
→ lease expires
→ second worker reclaims
→ event completes
```

---

## Test 3 — Duplicate job source

```text
same vacancy from two sources
→ one logical job
→ two job_sources
→ source disappearance from one provider
→ job remains active
→ disappearance from last active provider
→ job closes
```

---

## Test 4 — Application button

```text
existing match
→ Telegram Apply callback
→ application inserted
→ application.requested event emitted
→ callback returns immediately
```

---

## Test 5 — Successful application

```text
application.requested
→ fake provider Prepare
→ all fields available
→ fake provider Submit
→ application status submitted
→ submitted_at stored
→ Telegram success notification
```

---

## Test 6 — Missing input

```text
application requested
→ provider asks unknown required question
→ status needs_input
→ Telegram question sent
→ user answers callback
→ application.input_provided emitted
→ worker resumes
→ application submitted
```

---

## Test 7 — Manual required

```text
provider detects CAPTCHA
→ status manual_required
→ no automatic retry loop
→ Telegram manual message sent
```

---

## Test 8 — Duplicate Apply clicks

```text
Apply clicked 5 times
→ exactly one job_applications row
→ at most one active submission attempt
```

---

## Test 9 — Application worker crash

```text
application claimed
→ process dies before submit
→ lease expires
→ worker resumes
```

If provider submission outcome is uncertain, do not blindly resubmit without provider-specific idempotency/reconciliation logic.

---

# Block 28 — Failure Classification

Every provider error must map to one of these categories:

```text
temporary
rate_limited
validation
missing_input
manual_required
unsupported
job_closed
authentication
provider_error
unknown
```

Suggested policy:

```text
temporary       → retry
rate_limited    → retry using Retry-After/backoff
validation      → fail or needs_input
missing_input   → needs_input
manual_required → manual_required
unsupported     → manual_required
job_closed      → cancelled/failed terminal
authentication  → manual_required
provider_error  → retry limited times
unknown         → retry limited times then fail
```

---

# Block 29 — Security Requirements

Application automation processes sensitive personal data.

Required:

```text
no PII in logs
no resume body in logs
no authorization tokens in logs
no application answers in metrics
no Telegram bot token in errors
no provider secrets in DB plaintext unless required
```

Validate every external URL before server-side requests.

Reuse existing SSRF protections where possible.

For browser-worker navigation:

```text
allow http/https only
block localhost
block RFC1918/private networks
block metadata service IPs
block file://
block custom protocols
```

---

# Block 30 — Do Not Do Yet

Do not introduce these until the deterministic E2E path works:

```text
Kafka
NATS
RabbitMQ
Temporal
Elasticsearch
pgvector
LLM-only matching
AI agents controlling the entire workflow
generic arbitrary web crawler
CAPTCHA bypass
automatic free-text hallucinated application answers
automatic cover letters for every vacancy
complex ML ranking
microservice split
```

They can be added later if real measurements justify them.

---

# Block 31 — Optional Semantic Ranking

Only after deterministic matching quality is measured.

Possible future flow:

```text
hard filters
→ deterministic score
→ top N candidates
→ embedding similarity
→ optional semantic reranking
```

Never allow semantic ranking to bypass hard eligibility constraints.

Store semantic score separately from deterministic score.

Example:

```text
deterministic_score
semantic_score
final_score
confidence
```

---

# Block 32 — Implementation Order

Execute blocks in this order.

## Phase A — Reliability

```text
1. Outbox lease model
2. Notification lease model
3. Remove external I/O from DB transactions
4. Dead-letter handling
5. Crash recovery tests
```

---

## Phase B — Matching correctness

```text
6. Match versions
7. Chunked profile rematching
8. Candidate preselection for job matching
9. Job family classification
10. Better seniority scoring
11. Better skills scoring
12. Match confidence
```

---

## Phase C — Application foundation

```text
13. jobapplication domain
14. application schema
15. application profile
16. application provider interface
17. application worker
18. Telegram Apply callback
```

---

## Phase D — First working auto-apply

```text
19. first provider adapter
20. successful submission flow
21. needs_input flow
22. manual_required flow
23. Telegram application status updates
```

At the end of this phase the product must support:

```text
job discovered
→ matched
→ Telegram message
→ Apply pressed
→ real external application submitted
→ Telegram confirmation
```

for at least one supported ATS.

---

## Phase E — Coverage

```text
24. provider coverage metrics
25. second ATS application adapter
26. third ATS application adapter
27. more source connectors
28. browser fallback
```

---

## Phase F — Optimization

```text
29. runtime modes
30. independent worker scaling
31. matching performance optimization
32. application performance optimization
33. optional semantic reranking
```

---

# Block 33 — Definition of Done

HireRadar E2E implementation is complete when all of the following are true.

## Discovery

- New supported companies can be discovered automatically.
- Supported ATS sources are registered idempotently.
- Provider coverage is measurable.

## Ingestion

- Source jobs are persisted as raw snapshots.
- Jobs are normalized.
- Jobs are deduplicated without unsafe aggressive merging.
- Missing jobs close correctly only after source-specific rules are satisfied.

## Matching

- Matching is deterministic.
- Matching is restart-safe.
- More than 5000 jobs cannot corrupt user matches.
- New jobs do not trigger pathological SQL N+1 behavior.
- Match result contains score and confidence.
- Match explanation is persisted.

## Telegram

- User receives new high-quality matches.
- Notifications are retryable and restart-safe.
- Telegram network calls happen outside database transactions.
- User can Open, Save, Hide, and Apply.

## Applications

- Apply is idempotent.
- One user/job pair cannot create duplicate applications.
- Submission happens asynchronously.
- At least one real ATS provider is supported.
- Missing required data creates `needs_input`.
- CAPTCHA/auth/manual barriers create `manual_required`.
- Successful submission creates `submitted`.
- User receives Telegram result.

## Reliability

- Worker crashes do not lose work.
- Expired leases are reclaimed.
- Poison jobs eventually move to failed/dead-letter state.
- Failed work can be inspected and retried.

## Testing

- Unit tests cover matching, application state transitions, retry classification, and provider mapping.
- Integration tests cover PostgreSQL workflow.
- E2E tests cover source → Telegram.
- E2E tests cover Telegram Apply → submitted.
- CI runs automatically for every pull request.

---

# Block 34 — AI Agent Execution Rules

When implementing this plan:

1. Work block by block.
2. Do not redesign unrelated modules.
3. Preserve existing public behavior unless the block explicitly changes it.
4. Add migrations instead of editing already-applied migrations.
5. Keep domain logic separate from PostgreSQL and HTTP adapters.
6. Keep interfaces small and defined by the consuming package.
7. Do not add abstractions before at least one concrete use requires them.
8. Prefer explicit state machines over implicit boolean flags.
9. Every async state transition must be idempotent.
10. Every new background worker requires crash/retry tests.
11. Never hold DB transactions across network calls.
12. Never silently fabricate answers for job applications.
13. Never bypass CAPTCHA or authentication barriers.
14. Update documentation together with architecture changes.
15. Update OpenAPI and generated frontend types in the same commit as API changes.
16. Keep the project buildable after every block.
17. Run the complete relevant test suite after each block.
18. Do not begin the next phase while acceptance criteria for the current phase are failing.

---

# Block 35 — Expected Final User Flow

```text
User uploads resume
        │
        ▼
Resume parsed
        │
        ▼
Candidate profile built
        │
        ▼
HireRadar discovers companies and sources
        │
        ▼
Vacancies continuously collected
        │
        ▼
Jobs normalized and deduplicated
        │
        ▼
Candidate matching runs
        │
        ▼
High-quality match created
        │
        ▼
Telegram notification
        │
        ▼
User presses Apply
        │
        ▼
Application queued
        │
        ▼
Form prepared
        │
        ├───────────────┐
        │               │
        ▼               ▼
all data known     missing required data
        │               │
        │               ▼
        │          Telegram question
        │               │
        │          user answers
        │               │
        └───────┬───────┘
                ▼
          application submit
                │
       ┌────────┼─────────┐
       ▼        ▼         ▼
   submitted  manual    failed
       │      required     │
       └────────┼──────────┘
                ▼
             Telegram
```

This is the target E2E architecture.
