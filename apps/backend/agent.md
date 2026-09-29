@agentx

# Backend Architecture — Go + Hexagonal Architecture

## 1. Цель системы

Backend — центральная часть системы поиска и доставки релевантных remote-вакансий.

Основной pipeline:

```text
Sources
    ↓
Job Ingestion
    ↓
Normalization
    ↓
Deduplication
    ↓
Classification
    ↓
Persistence
    ↓
Candidate Matching
    ↓
Notification Queue
    ↓
Telegram
```

Пользовательский flow:

```text
Registration
    ↓
Profile
    ↓
Resume Upload
    ↓
Resume Parsing
    ↓
Skills / Experience
    ↓
Job Preferences
    ↓
Source Preferences
    ↓
Telegram Connection
    ↓
Job Matching
    ↓
Telegram Notifications
```

---

# 2. Технологический стек

```text
Language: Go

HTTP:
net/http
chi

Database:
PostgreSQL
pgx

Migrations:
goose

Cache / Queue:
Redis

Storage:
S3-compatible storage

Telegram:
Telegram Bot API

Configuration:
environment variables

Observability:
slog
OpenTelemetry
Prometheus

Deployment:
Docker
Railway
```

Опционально:

```text
sqlc
```

для типизированного SQL.

ORM уровня GORM для core persistence не нужен.

---

# 3. Архитектурный подход

Используется:

```text
Hexagonal Architecture
+
Domain Driven Design principles
+
Modular Monolith
```

Не нужно начинать с микросервисов.

Backend представляет собой один кодовый репозиторий с несколькими executable:

```text
api
worker
scheduler
telegram
```

Они используют одни domain/application packages.

---

# 4. Hexagonal Architecture

Основное правило:

```text
Adapters
    ↓
Application
    ↓
Domain
```

Dependency direction:

```text
Infrastructure ────────┐
                      ↓
Adapters → Application → Domain
                      ↑
Infrastructure ────────┘
```

Domain ничего не знает про:

```text
PostgreSQL
Redis
HTTP
Telegram
S3
Greenhouse
Lever
Ashby
GitHub
Railway
```

Application работает через ports.

Infrastructure реализует эти ports.

---

# 5. Структура проекта

```text
.
├── cmd/
│   ├── api/
│   │   └── main.go
│   │
│   ├── worker/
│   │   └── main.go
│   │
│   ├── scheduler/
│   │   └── main.go
│   │
│   └── telegram/
│       └── main.go
│
├── internal/
│   │
│   ├── auth/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── user/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── profile/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── resume/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── telegram/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── source/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── company/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── job/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── matching/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   ├── notification/
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   │
│   └── feedback/
│       ├── domain/
│       ├── application/
│       └── adapters/
│
├── pkg/
│   ├── postgres/
│   ├── redis/
│   ├── s3/
│   ├── httpx/
│   ├── observability/
│   └── clock/
│
├── migrations/
│
├── deployments/
│
├── go.mod
├── go.sum
└── Makefile
```

---

# 6. Структура bounded context

Каждый модуль:

```text
job/
├── domain/
│   ├── job.go
│   ├── company.go
│   ├── skill.go
│   ├── repository.go
│   ├── errors.go
│   └── service.go
│
├── application/
│   ├── commands/
│   ├── queries/
│   ├── ports/
│   └── dto/
│
└── adapters/
    ├── postgres/
    ├── http/
    └── redis/
```

---

# 7. Domain Layer

Domain содержит:

```text
Entities
Value Objects
Domain Services
Domain Events
Repository interfaces
Business Rules
```

Domain не должен импортировать infrastructure packages.

---

# 8. User Domain

Основная entity:

```go
type User struct {
	ID            UserID
	Email         Email
	PasswordHash  string
	Status        Status
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
```

Status:

```go
type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
	StatusDeleted Status = "deleted"
)
```

---

# 9. User Repository Port

```go
type Repository interface {
	Create(ctx context.Context, user *User) error

	ByID(ctx context.Context, id UserID) (*User, error)
	ByEmail(ctx context.Context, email Email) (*User, error)

	Update(ctx context.Context, user *User) error

	ExistsByEmail(ctx context.Context, email Email) (bool, error)
}
```

PostgreSQL implementation находится:

```text
internal/user/adapters/postgres
```

---

# 10. Authentication

Поддерживаем:

```text
Register
Login
Refresh
Logout
LogoutAll
VerifyEmail
ForgotPassword
ResetPassword
```

---

# 11. Registration Flow

```text
POST /api/v1/auth/register

        ↓

Validate request

        ↓

Normalize email

        ↓

Check existing user

        ↓

Hash password

        ↓

Create User

        ↓

Create email verification token

        ↓

Commit transaction

        ↓

Publish UserRegistered

        ↓

Send verification email
```

Application command:

```go
type RegisterCommand struct {
	Email    string
	Password string
}
```

Handler:

```go
type RegisterHandler struct {
	users      user.Repository
	hasher     PasswordHasher
	tokens     VerificationTokenRepository
	tx         TransactionManager
	dispatcher EventDispatcher
}
```

---

# 12. Password Hashing Port

```go
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash string, password string) error
}
```

Implementation:

```text
Argon2id
```

Domain/Application не знает конкретный алгоритм.

---

# 13. Session Model

```go
type Session struct {
	ID               SessionID
	UserID           UserID
	RefreshTokenHash string

	UserAgent string
	IPAddress net.IP

	ExpiresAt time.Time
	RevokedAt *time.Time

	CreatedAt time.Time
}
```

---

# 14. Tokens

Используем:

```text
short-lived access token
+
opaque refresh token
```

Например:

```text
access: 15 minutes
refresh: 30 days
```

Refresh token хранится в БД только как hash.

---

# 15. Refresh Token Rotation

При:

```text
POST /auth/refresh
```

выполняется:

```text
validate refresh token
        ↓
find session
        ↓
check revoked
        ↓
check expiration
        ↓
revoke previous refresh token
        ↓
generate new refresh token
        ↓
update session
        ↓
generate new access token
```

Refresh token нельзя использовать повторно.

---

# 16. Profile Domain

```go
type Profile struct {
	UserID UserID

	FirstName string
	LastName  string

	Country  CountryCode
	City     string
	Timezone string

	ExperienceYears int
	Seniority       Seniority

	DesiredSalary *Money

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

---

# 17. Seniority

```go
type Seniority string

const (
	SeniorityIntern    Seniority = "intern"
	SeniorityJunior    Seniority = "junior"
	SeniorityMiddle    Seniority = "middle"
	SenioritySenior    Seniority = "senior"
	SeniorityStaff     Seniority = "staff"
	SeniorityPrincipal Seniority = "principal"
	SeniorityLead      Seniority = "lead"
)
```

---

# 18. User Skills

Skill является нормализованной entity.

```go
type Skill struct {
	ID   SkillID
	Name string

	NormalizedName string
}
```

Связь пользователя:

```go
type UserSkill struct {
	UserID  UserID
	SkillID SkillID

	Years *float64
	Level SkillLevel

	Source SkillSource

	Confirmed bool
}
```

Source:

```text
manual
resume
inferred
```

---

# 19. Skill Aliases

Критически важно для matching.

```text
golang → Go
go lang → Go
react.js → React
reactjs → React
postgres → PostgreSQL
postgresql → PostgreSQL
next → Next.js
nextjs → Next.js
k8s → Kubernetes
js → JavaScript
ts → TypeScript
```

Таблица:

```text
skill_aliases
```

```go
type SkillAlias struct {
	ID      int64
	SkillID SkillID
	Alias   string
}
```

---

# 20. Desired Positions

```go
type DesiredPosition struct {
	ID     PositionID
	UserID UserID

	Title           string
	NormalizedTitle string
}
```

Например:

```text
Go Developer
Golang Developer
Backend Engineer
Software Engineer
Frontend Developer
Full Stack Engineer
```

---

# 21. Job Preferences

```go
type JobPreferences struct {
	UserID UserID

	RemotePolicies []RemotePolicy
	EmploymentTypes []EmploymentType

	AllowedRegions []Region
	ExcludedCountries []CountryCode

	MinimumSalary *Money

	MinimumMatchScore int

	MaximumJobAge time.Duration

	NotificationsEnabled bool
}
```

---

# 22. Employment Types

```go
type EmploymentType string

const (
	EmploymentFullTime   EmploymentType = "full_time"
	EmploymentPartTime   EmploymentType = "part_time"
	EmploymentContract   EmploymentType = "contract"
	EmploymentFreelance  EmploymentType = "freelance"
	EmploymentB2B        EmploymentType = "b2b"
	EmploymentInternship EmploymentType = "internship"
)
```

---

# 23. Resume Domain

Пользователь может иметь несколько CV.

```go
type Resume struct {
	ID     ResumeID
	UserID UserID

	FileName string
	ObjectKey string

	ContentType string
	Size        int64
	SHA256      string

	Status ResumeStatus

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

---

# 24. Resume Status

```text
uploaded
processing
processed
failed
deleted
```

---

# 25. Resume Upload

Flow:

```text
Frontend
   ↓
POST /resumes/upload-url
   ↓
Backend
   ↓
Generate presigned S3 URL
   ↓
Frontend uploads directly to S3
   ↓
POST /resumes/{id}/complete
   ↓
Backend verifies object
   ↓
ResumeUploaded event
   ↓
Worker
   ↓
Resume processing
```

Не нужно проксировать большие PDF через API.

---

# 26. Storage Port

```go
type ObjectStorage interface {
	PresignUpload(
		ctx context.Context,
		key string,
		contentType string,
		expiration time.Duration,
	) (string, error)

	PresignDownload(
		ctx context.Context,
		key string,
		expiration time.Duration,
	) (string, error)

	Stat(
		ctx context.Context,
		key string,
	) (ObjectInfo, error)

	Delete(
		ctx context.Context,
		key string,
	) error
}
```

Adapter может быть:

```text
Railway Bucket
RustFS
AWS S3
Cloudflare R2
MinIO
```

---

# 27. Resume Processing

Worker:

```text
ResumeUploaded

        ↓

download object

        ↓

validate MIME

        ↓

extract text

        ↓

normalize text

        ↓

extract:
    skills
    positions
    companies
    experience
    education
    languages

        ↓

save parsed resume

        ↓

generate profile suggestions
```

---

# 28. Parsed Resume

```go
type ParsedResume struct {
	ResumeID ResumeID

	Text string

	Skills []DetectedSkill

	Positions []DetectedPosition

	Experience []Experience

	Languages []Language

	TotalExperienceMonths int
}
```

---

# 29. Resume → Profile

Резюме не должно автоматически менять профиль.

Вместо этого:

```text
Resume Parser
      ↓
Profile Suggestions
      ↓
User reviews
      ↓
Accept / Reject
```

Например:

```json
{
  "suggestions": {
    "skills": [
      {
        "name": "Go",
        "confidence": 0.97
      },
      {
        "name": "PostgreSQL",
        "confidence": 0.92
      }
    ]
  }
}
```

---

# 30. Source Domain

Source — внешний источник данных.

```go
type Source struct {
	ID SourceID

	Name string
	Type SourceType

	Enabled bool

	Priority int

	SyncInterval time.Duration

	LastSyncAt *time.Time
	NextSyncAt *time.Time

	Config SourceConfig

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

---

# 31. Source Types

```text
greenhouse
lever
ashby
workable
teamtailor
smartrecruiters

github_repository
github_issues

rss
api

job_board

company_careers

html
```

---

# 32. Source Registry

Source Registry содержит информацию:

```text
Grafana
    ↓
type: greenhouse
board: grafana
enabled: true

Company X
    ↓
type: lever
tenant: company-x

RemoteInTech
    ↓
type: github_repository
repo: remoteintech/remote-jobs
```

---

# 33. Source Adapter Port

Ключевой интерфейс:

```go
type JobSource interface {
	Fetch(
		ctx context.Context,
		cursor Cursor,
	) (FetchResult, error)
}
```

```go
type FetchResult struct {
	Jobs       []ExternalJob
	NextCursor *Cursor
}
```

---

# 34. External Job

Каждый connector возвращает одну модель:

```go
type ExternalJob struct {
	ExternalID string

	CompanyName string

	Title       string
	Description string

	Location string

	EmploymentType string

	Salary *ExternalSalary

	ApplyURL string

	PublishedAt *time.Time

	Raw json.RawMessage
}
```

Таким образом:

```text
Greenhouse
Lever
Ashby
RemoteOK
GitHub
RSS
```

возвращают одинаковую структуру.

---

# 35. Greenhouse Adapter

```text
internal/source/adapters/greenhouse
```

Реализует:

```go
type Adapter struct {
	client HTTPClient
	config Config
}

func (a *Adapter) Fetch(
	ctx context.Context,
	cursor source.Cursor,
) (source.FetchResult, error)
```

---

# 36. Lever Adapter

Аналогично:

```text
internal/source/adapters/lever
```

---

# 37. Ashby Adapter

```text
internal/source/adapters/ashby
```

---

# 38. GitHub Repository Discovery

GitHub curated repositories не считаются главным источником вакансий.

Они являются:

```text
discovery sources
```

Pipeline:

```text
remoteintech
awesome-remote-job
established-remote
global-hiring

        ↓

GitHub Adapter

        ↓

extract companies

        ↓

extract career URLs

        ↓

detect ATS

        ↓

create/update Sources
```

---

# 39. ATS Detection

Например нашли:

```text
https://boards.greenhouse.io/company
```

Detector определяет:

```text
ATS = Greenhouse
tenant = company
```

И автоматически создаёт:

```text
Source {
    Type: greenhouse
}
```

---

# 40. ATS Detector Port

```go
type ATSDetector interface {
	Detect(
		ctx context.Context,
		careerURL string,
	) (*DetectedATS, error)
}
```

```go
type DetectedATS struct {
	Type   ATSType
	Tenant string
	URL    string
}
```

---

# 41. Source Scheduler

Scheduler периодически ищет sources:

```sql
WHERE enabled = true
AND next_sync_at <= NOW()
```

И отправляет:

```text
SyncSource(sourceID)
```

в очередь.

---

# 42. Интервалы

Пример:

```text
Greenhouse     15 min
Lever          15 min
Ashby          15 min

RSS            30 min

Job boards     30-60 min

HTML           1-3 hours

GitHub lists   24 hours
```

Интервалы должны храниться в Source.

Не hardcode.

---

# 43. Source Sync

Flow:

```text
SyncSource
     ↓
Load Source
     ↓
Resolve Adapter
     ↓
Fetch
     ↓
Save raw jobs
     ↓
Normalize
     ↓
Deduplicate
     ↓
Upsert jobs
     ↓
Publish JobCreated / JobUpdated
     ↓
Update source cursor
     ↓
Update last_sync_at
```

---

# 44. Source Sync Runs

Каждый запуск логируем:

```text
source_sync_runs
```

```go
type SyncRun struct {
	ID SourceSyncRunID

	SourceID SourceID

	Status SyncStatus

	StartedAt  time.Time
	FinishedAt *time.Time

	FetchedJobs int
	NewJobs     int
	UpdatedJobs int

	ErrorMessage *string
}
```

---

# 45. Raw Jobs

Обязательно сохранять исходные данные.

```text
raw_jobs
```

```go
type RawJob struct {
	ID RawJobID

	SourceID SourceID

	ExternalID string

	Payload []byte

	ContentHash string

	FetchedAt time.Time
}
```

Это позволяет:

* дебажить parser;
* повторно нормализовать данные;
* расследовать ошибки;
* менять normalization без повторного запроса источника.

---

# 46. Job Domain

Нормализованная vacancy:

```go
type Job struct {
	ID JobID

	CompanyID CompanyID

	Title           string
	NormalizedTitle string

	Description string

	Seniority Seniority

	EmploymentTypes []EmploymentType

	RemotePolicy RemotePolicy

	Location JobLocation

	Salary *SalaryRange

	PublishedAt *time.Time

	FirstSeenAt time.Time
	LastSeenAt  time.Time

	Status JobStatus

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

---

# 47. Job Status

```text
active
closed
expired
unknown
```

---

# 48. Remote Policy

```text
worldwide
remote
remote_region
remote_country
hybrid
onsite
unknown
```

---

# 49. Job Location

```go
type JobLocation struct {
	RemotePolicy RemotePolicy

	Countries []CountryCode
	Regions   []Region

	ExcludedCountries []CountryCode

	Timezones []string
}
```

Это критически важно.

`remote=true` недостаточно.

---

# 50. Location Rules

Система должна понимать:

```text
Worldwide

Global

Anywhere

EMEA

Europe only

EU only

US only

Canada only

LATAM

UTC ±3

GMT+0 — GMT+4
```

---

# 51. Kazakhstan Eligibility

Для каждого Job желательно вычислять:

```go
type Eligibility string

const (
	Eligible     Eligibility = "eligible"
	NotEligible  Eligibility = "not_eligible"
	Unknown      Eligibility = "unknown"
)
```

Не превращать `unknown` автоматически в `not_eligible`.

---

# 52. Company

```go
type Company struct {
	ID CompanyID

	Name           string
	NormalizedName string

	WebsiteURL string
	CareersURL string

	RemotePolicy RemotePolicy

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

---

# 53. Job Skills

После normalization:

```text
job_skills
```

```go
type JobSkill struct {
	JobID   JobID
	SkillID SkillID

	Required bool

	Confidence float64
}
```

---

# 54. Job Normalization

Normalization pipeline:

```text
ExternalJob
     ↓
HTML cleanup
     ↓
Title normalization
     ↓
Company normalization
     ↓
Location normalization
     ↓
Employment normalization
     ↓
Salary normalization
     ↓
Skill extraction
     ↓
Seniority extraction
     ↓
Job
```

---

# 55. Title Normalization

Например:

```text
Sr. Golang Software Engineer
Senior Go Developer
Senior Backend Engineer (Golang)
```

не превращаем в одну строку полностью.

Храним:

```text
original title
+
normalized attributes
```

Например:

```text
title:
Senior Backend Engineer (Golang)

normalized_role:
backend_engineer

seniority:
senior

skills:
Go
```

---

# 56. Salary

```go
type Money struct {
	Amount   int64
	Currency Currency
}

type SalaryRange struct {
	Min *Money
	Max *Money

	Period SalaryPeriod
}
```

Period:

```text
hour
day
month
year
```

Нельзя терять original salary.

---

# 57. Job Sources

Одна vacancy может присутствовать:

```text
Company Careers
Greenhouse
RemoteOK
Himalayas
GitHub
```

Поэтому:

```text
jobs
```

и:

```text
job_sources
```

разделены.

```go
type JobSourceReference struct {
	JobID JobID

	SourceID SourceID

	ExternalID string
	URL        string

	FirstSeenAt time.Time
	LastSeenAt  time.Time
}
```

---

# 58. Deduplication

Deduplication работает несколькими уровнями.

## Level 1

```text
source_id + external_id
```

## Level 2

Canonical apply URL.

## Level 3

```text
company
+
normalized title
+
location
```

## Level 4

Description fingerprint.

---

# 59. Fingerprint

Можно строить:

```text
SHA256(
    normalized_company +
    normalized_title +
    normalized_location
)
```

Но fingerprint не должен быть единственным механизмом.

---

# 60. Duplicate Confidence

```go
type DuplicateResult struct {
	JobID      JobID
	Confidence float64
	Reason     DuplicateReason
}
```

Например:

```text
1.00 external_id
0.99 canonical_url
0.95 company + title + location
0.87 content similarity
```

---

# 61. Canonical Source

Если одна vacancy найдена в нескольких местах:

```text
Company Careers
Greenhouse
RemoteOK
```

предпочитаем:

```text
official company source
        ↓
ATS
        ↓
job board
        ↓
aggregator
```

---

# 62. Detecting Closed Jobs

Во время sync источник возвращает актуальный набор.

Если ранее активная vacancy больше не существует:

```text
active
↓
missing
↓
verification
↓
closed
```

Не закрывать после одного сетевого сбоя.

Использовать:

```text
missing_count
```

Например:

```text
missing 1 sync → active
missing 2 sync → active
missing 3 sync → closed
```

Для API с authoritative full listing можно использовать более строгую стратегию.

---

# 63. Matching Domain

Matching является отдельным bounded context.

Он не должен находиться внутри Job.

```text
Job
+
Candidate Profile
+
Preferences
        ↓
Matching Engine
        ↓
JobMatch
```

---

# 64. Matching Pipeline

```text
JobCreated
     ↓
Candidate Preselection
     ↓
Hard Filters
     ↓
Rule-based Score
     ↓
Semantic/AI Score optional
     ↓
Final Score
     ↓
Persist Match
     ↓
Notification Decision
```

---

# 65. Hard Filters

Сначала дешёвые deterministic правила.

```text
job active?
source enabled?
country allowed?
remote requirement satisfied?
employment type accepted?
salary acceptable?
job not hidden?
job not already applied?
```

Если hard filter провален:

```text
match = rejected
```

Дальнейший scoring не нужен.

---

# 66. Source Preferences

Пользователь может включать/отключать источники.

```text
user_source_preferences

user_id
source_id

enabled
```

По умолчанию можно использовать глобальные enabled sources.

---

# 67. Candidate Preselection

Нельзя при каждой новой вакансии сравнивать её со всеми пользователями.

Например:

```text
100 000 users
×
10 000 jobs

= 1 billion comparisons
```

Сначала делаем candidate retrieval.

Фильтры:

```text
country
remote preference
employment type
desired position
skills
seniority
```

После этого остаётся небольшой candidate set.

---

# 68. Matching Score

Для MVP:

```text
score =
    skill_score       * 0.40 +
    position_score    * 0.25 +
    seniority_score   * 0.10 +
    location_score    * 0.10 +
    employment_score  * 0.05 +
    salary_score      * 0.05 +
    freshness_score   * 0.05
```

Весовые коэффициенты должны быть configurable.

---

# 69. Skill Score

Например:

User:

```text
Go
PostgreSQL
Docker
Redis
React
```

Job:

```text
Go
PostgreSQL
Docker
Kubernetes
AWS
```

Получаем:

```text
matched:
Go
PostgreSQL
Docker

missing:
Kubernetes
AWS
```

Skill score вычисляется отдельно.

---

# 70. Required / Optional Skills

Нужно различать:

```text
required
preferred
nice_to_have
```

Например отсутствие:

```text
Go
```

может сильно уменьшать score.

Отсутствие:

```text
Terraform nice-to-have
```

не должно уничтожать match.

---

# 71. Job Match

```go
type JobMatch struct {
	ID JobMatchID

	UserID UserID
	JobID  JobID

	Score float64

	SkillScore      float64
	PositionScore   float64
	SeniorityScore  float64
	LocationScore   float64
	EmploymentScore float64
	SalaryScore     float64
	FreshnessScore  float64

	Status MatchStatus

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

---

# 72. Match Status

```text
new
notified
viewed
saved
hidden
applied
rejected
expired
```

---

# 73. Match Explanation

Отдельно сохраняем:

```go
type MatchExplanation struct {
	MatchedSkills []string
	MissingSkills []string

	Reasons []string
}
```

Frontend/Telegram сможет показать:

```text
92% match

Matched:
Go
PostgreSQL
Docker
Redis

Missing:
Kubernetes
```

---

# 74. AI Layer

AI не должен быть обязательным для core pipeline.

Правильная схема:

```text
Hard filters
    ↓
Deterministic matching
    ↓
AI enrichment/reranking
```

Если AI provider недоступен:

```text
система продолжает работать
```

---

# 75. AI Port

```go
type JobAnalyzer interface {
	Analyze(
		ctx context.Context,
		profile CandidateProfile,
		job Job,
	) (Analysis, error)
}
```

Application не знает:

```text
OpenAI
Anthropic
Gemini
local model
```

---

# 76. Embeddings

В будущем можно добавить:

```text
PostgreSQL
+
pgvector
```

Хранить:

```text
resume_embedding
profile_embedding
job_embedding
```

Использовать для candidate retrieval / semantic matching.

Но это не требуется для первой версии.

---

# 77. Telegram Linking

Телефон пользователя не нужен.

Flow:

```text
Web
↓
Connect Telegram
↓
POST /telegram/link
↓
Backend creates one-time token
↓
Frontend opens:

t.me/job_bot?start=TOKEN

↓
User presses START
↓
Bot receives token
↓
Backend validates token
↓
Telegram account attached
```

---

# 78. Telegram Link Token

```go
type LinkToken struct {
	ID TokenID

	UserID UserID

	TokenHash string

	ExpiresAt time.Time
	UsedAt    *time.Time

	CreatedAt time.Time
}
```

TTL:

```text
10-15 minutes
```

Token:

```text
single use
```

---

# 79. Telegram Account

```go
type TelegramAccount struct {
	ID TelegramAccountID

	UserID UserID

	TelegramUserID int64
	ChatID         int64

	Username *string

	Status TelegramStatus

	ConnectedAt time.Time
	UpdatedAt   time.Time
}
```

---

# 80. Telegram Webhook

Production:

```text
Telegram
    ↓
POST /webhooks/telegram
    ↓
Telegram Adapter
    ↓
Application Command
```

Polling для production не нужен.

---

# 81. Telegram Commands

Минимально:

```text
/start
/settings
/jobs
/pause
/resume
/unlink
```

Основная конфигурация остаётся в web application.

Bot является delivery/interface adapter.

---

# 82. Notifications Domain

Telegram не должен быть напрямую встроен в Matching.

Matching генерирует:

```text
JobMatched
```

Notification application решает:

```text
надо ли уведомлять?
каким каналом?
когда?
```

---

# 83. Notification

```go
type Notification struct {
	ID NotificationID

	UserID UserID

	Type NotificationType
	Channel NotificationChannel

	ReferenceID string

	Status NotificationStatus

	ScheduledAt time.Time
	SentAt      *time.Time

	Attempts int

	LastError *string

	CreatedAt time.Time
}
```

---

# 84. Notification Channels

Сейчас:

```text
telegram
```

В будущем:

```text
email
push
webhook
```

---

# 85. Telegram Notification

Пример:

```text
92% match

Senior Go Engineer
Acme

Worldwide
Contract
$6,000–8,000/month

Go · PostgreSQL · Docker · Kubernetes

Published 42 minutes ago

Matched:
Go
PostgreSQL
Docker

[Open]
[Save]
[Not interested]
```

---

# 86. Telegram Actions

Inline callback:

```text
job:open:{matchID}
job:save:{matchID}
job:hide:{matchID}
job:applied:{matchID}
```

Никогда не доверять данным callback без проверки:

```text
match belongs to Telegram user
```

---

# 87. Notification Preferences

```go
type NotificationPreferences struct {
	UserID UserID

	Enabled bool

	MinimumScore float64

	Immediate bool

	DigestEnabled bool

	QuietHours *QuietHours

	MaxPerDay *int
}
```

---

# 88. Quiet Hours

Например:

```text
23:00 → 08:00
Asia/Almaty
```

Match сохраняется сразу.

Notification:

```text
scheduled_at = next allowed time
```

---

# 89. Notification Deduplication

Один:

```text
user_id + job_id + notification_type
```

не должен отправляться повторно без причины.

Использовать DB unique constraint.

---

# 90. Feedback

Пользователь может:

```text
save
hide
not interested
applied
```

Feedback:

```go
type Feedback struct {
	ID FeedbackID

	UserID UserID
	JobID  JobID

	Type FeedbackType

	Reason *FeedbackReason

	CreatedAt time.Time
}
```

---

# 91. Feedback Reasons

Например:

```text
wrong_stack
wrong_location
wrong_seniority
salary_too_low
not_remote
not_interested_company
not_interested_role
duplicate
```

Это намного полезнее простого dislike.

---

# 92. Feedback → Matching

Позже можно учитывать feedback:

```text
User repeatedly hides Java jobs
↓
decrease Java relevance

User saves Go infrastructure jobs
↓
increase Go/platform relevance
```

Но пользовательские explicit preferences всегда важнее inferred preferences.

---

# 93. Saved Jobs

```text
saved_jobs

user_id
job_id
created_at
```

Unique:

```text
(user_id, job_id)
```

---

# 94. Applied Jobs

Пользователь вручную отмечает:

```text
applied
```

Храним:

```text
user_job_applications

user_id
job_id

status
applied_at

notes
```

В будущем:

```text
applied
screening
interview
offer
rejected
withdrawn
```

---

# 95. Event Architecture

Внутри modular monolith используем domain/application events.

Примеры:

```text
UserRegistered
ResumeUploaded
ResumeProcessed

SourceDiscovered
SourceSynced

JobCreated
JobUpdated
JobClosed

JobMatched

NotificationRequested
NotificationSent
NotificationFailed

TelegramConnected
TelegramDisconnected
```

---

# 96. Transactional Outbox

Для важных asynchronous events использовать:

```text
Transactional Outbox Pattern
```

Например:

```text
BEGIN

INSERT jobs

INSERT outbox_events (
    type = 'JobCreated'
)

COMMIT
```

Worker затем читает outbox.

Так мы не получаем ситуацию:

```text
job committed
↓
process crashed
↓
event lost
```

---

# 97. Outbox Table

```text
outbox_events

id UUID

aggregate_type
aggregate_id

event_type

payload JSONB

created_at
processed_at

attempts
last_error
```

---

# 98. Queue

Redis используется для:

```text
job ingestion tasks
resume processing
matching
notification delivery
source discovery
```

Но PostgreSQL остаётся source of truth.

Redis не должен быть единственным местом хранения важного состояния.

---

# 99. Idempotency

Все background jobs должны быть idempotent.

Например повторный:

```text
SyncSource(sourceID)
```

не должен создавать дубли.

Повторный:

```text
SendNotification(notificationID)
```

не должен отправить пользователю 10 одинаковых сообщений.

---

# 100. Retry Policy

External integrations:

```text
1 attempt
↓
5 sec
↓
30 sec
↓
2 min
↓
10 min
```

с:

```text
exponential backoff
+
jitter
```

---

# 101. Dead Letter

После максимального количества попыток:

```text
failed_jobs
```

или статус:

```text
dead
```

Оператор должен видеть причину.

---

# 102. Rate Limiting

Source adapters должны учитывать:

```text
rate limit
retry-after
429
timeouts
```

Каждый source имеет configurable:

```text
requests_per_second
burst
timeout
```

---

# 103. HTTP Client Port

```go
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
```

Infrastructure wrapper добавляет:

```text
timeouts
retry
metrics
tracing
rate limiting
user-agent
```

---

# 104. Context

Каждая external операция принимает:

```go
context.Context
```

Никаких:

```go
context.Background()
```

глубоко внутри request flow без веской причины.

---

# 105. API

Base:

```text
/api/v1
```

---

# 106. Auth API

```text
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout
POST   /api/v1/auth/logout-all

POST   /api/v1/auth/verify-email
POST   /api/v1/auth/resend-verification

POST   /api/v1/auth/forgot-password
POST   /api/v1/auth/reset-password
```

---

# 107. Profile API

```text
GET    /api/v1/profile
PUT    /api/v1/profile

GET    /api/v1/profile/skills
PUT    /api/v1/profile/skills

GET    /api/v1/profile/positions
PUT    /api/v1/profile/positions

GET    /api/v1/profile/preferences
PUT    /api/v1/profile/preferences
```

---

# 108. Resume API

```text
GET    /api/v1/resumes

POST   /api/v1/resumes/upload-url
POST   /api/v1/resumes/{id}/complete

GET    /api/v1/resumes/{id}
DELETE /api/v1/resumes/{id}

GET    /api/v1/resumes/{id}/analysis

POST   /api/v1/resumes/{id}/suggestions/{suggestionID}/accept
POST   /api/v1/resumes/{id}/suggestions/{suggestionID}/reject
```

---

# 109. Telegram API

```text
GET    /api/v1/telegram
POST   /api/v1/telegram/link
DELETE /api/v1/telegram

PUT    /api/v1/telegram/preferences
```

Webhook:

```text
POST /webhooks/telegram
```

Webhook не находится под authenticated `/api/v1`.

---

# 110. Jobs API

```text
GET /api/v1/jobs
GET /api/v1/jobs/{id}
```

Filters:

```text
minimum_score
skills
employment_type
remote_policy
country
source
salary_min
published_after
status
```

---

# 111. User Job API

```text
GET    /api/v1/jobs/matches

POST   /api/v1/jobs/{id}/save
DELETE /api/v1/jobs/{id}/save

POST   /api/v1/jobs/{id}/hide
POST   /api/v1/jobs/{id}/applied
```

---

# 112. Sources API

Для пользователя:

```text
GET /api/v1/sources
PUT /api/v1/sources/{id}/preference
```

Admin:

```text
GET    /api/v1/admin/sources
POST   /api/v1/admin/sources
GET    /api/v1/admin/sources/{id}
PUT    /api/v1/admin/sources/{id}
DELETE /api/v1/admin/sources/{id}

POST /api/v1/admin/sources/{id}/sync
```

---

# 113. Job Feed

Основной endpoint:

```text
GET /api/v1/jobs/matches
```

Response:

```json
{
  "items": [
    {
      "job": {
        "id": "...",
        "title": "Senior Go Engineer",
        "company": "Acme",
        "remote_policy": "worldwide",
        "employment_types": ["contract"],
        "published_at": "..."
      },
      "match": {
        "score": 0.91,
        "matched_skills": [
          "Go",
          "PostgreSQL",
          "Docker"
        ],
        "missing_skills": [
          "Kubernetes"
        ]
      }
    }
  ],
  "next_cursor": "..."
}
```

---

# 114. Pagination

Для jobs использовать cursor pagination.

Не:

```text
OFFSET 50000
```

Использовать:

```text
cursor
limit
```

Например:

```text
created_at + id
```

---

# 115. PostgreSQL Schema

Основные таблицы:

```text
users
sessions

email_verification_tokens
password_reset_tokens

user_profiles

skills
skill_aliases
user_skills
user_positions

job_preferences
user_source_preferences

resumes
parsed_resumes
resume_suggestions

telegram_accounts
telegram_link_tokens

companies

sources
source_sync_runs
source_cursors

raw_jobs

jobs
job_locations
job_skills
job_sources

job_matches

saved_jobs
user_job_feedback
user_job_applications

notification_preferences
notifications

outbox_events
```

---

# 116. PostgreSQL Extensions

Рассмотреть:

```text
pg_trgm
citext
```

Позже:

```text
vector
```

---

# 117. Database Constraints

Бизнес-инварианты должны поддерживаться БД.

Например:

```text
users.email UNIQUE

skills.normalized_name UNIQUE

telegram_accounts.telegram_user_id UNIQUE

saved_jobs(user_id, job_id) UNIQUE

job_sources(source_id, external_id) UNIQUE

job_matches(user_id, job_id) UNIQUE
```

---

# 118. Transactions

Application layer определяет transaction boundaries.

Port:

```go
type TransactionManager interface {
	WithinTransaction(
		ctx context.Context,
		fn func(ctx context.Context) error,
	) error
}
```

Нельзя размазывать:

```text
tx.Begin()
tx.Commit()
```

по domain-коду.

---

# 119. Repository Rule

Repository определяется потребностями domain/application.

Не создавать generic:

```go
type Repository[T any] interface {
	Create(T)
	Update(T)
	Delete(T)
	Find(...)
}
```

Для каждого aggregate собственный port.

Например:

```go
type JobRepository interface {
	ByID(ctx context.Context, id JobID) (*Job, error)

	FindByExternalReference(
		ctx context.Context,
		sourceID SourceID,
		externalID string,
	) (*Job, error)

	Save(ctx context.Context, job *Job) error
}
```

---

# 120. SQL

Для сложных query:

```text
pgx
+
sqlc
```

предпочтительнее ORM.

Особенно для:

```text
candidate preselection
job feed
matching
source scheduling
analytics
```

---

# 121. Dependency Injection

Не нужен тяжёлый DI framework.

В:

```text
cmd/api/main.go
```

явно собираем зависимости:

```go
db := postgres.New(...)
redisClient := redis.New(...)
storage := s3.New(...)

userRepo := userpostgres.NewRepository(db)
jobRepo := jobpostgres.NewRepository(db)

registerHandler := auth.NewRegisterHandler(
	userRepo,
	hasher,
	...
)

server := httpapi.New(...)
```

Это composition root.

---

# 122. Config

```go
type Config struct {
	Environment string

	HTTP HTTPConfig

	Postgres PostgresConfig
	Redis    RedisConfig
	S3       S3Config

	Auth AuthConfig

	Telegram TelegramConfig
}
```

Config загружается при старте.

Если обязательной переменной нет:

```text
fail fast
```

---

# 123. Secrets

Через environment/secrets:

```text
DATABASE_URL
REDIS_URL

JWT_PRIVATE_KEY

S3_ENDPOINT
S3_BUCKET
S3_ACCESS_KEY
S3_SECRET_KEY

TELEGRAM_BOT_TOKEN
TELEGRAM_WEBHOOK_SECRET
```

Никаких secrets в Git.

---

# 124. Logging

Использовать:

```text
log/slog
```

Structured logging:

```json
{
  "level": "INFO",
  "service": "worker",
  "source_id": "...",
  "sync_run_id": "...",
  "jobs_received": 74,
  "duration_ms": 832
}
```

---

# 125. Request ID

Каждый HTTP request:

```text
X-Request-ID
```

Request ID прокидывается:

```text
HTTP
↓
Application
↓
DB
↓
External API
↓
Logs
```

---

# 126. Observability

Минимально:

```text
logs
metrics
traces
```

OpenTelemetry.

---

# 127. Metrics

Примеры:

```text
http_requests_total
http_request_duration

source_sync_total
source_sync_failed_total
source_sync_duration

jobs_ingested_total
jobs_created_total
jobs_deduplicated_total

matches_created_total

notifications_sent_total
notifications_failed_total

telegram_api_duration
```

---

# 128. Health Checks

```text
GET /health
GET /ready
```

`/health`:

```text
process alive
```

`/ready`:

```text
PostgreSQL reachable
Redis reachable
required dependencies ready
```

---

# 129. Graceful Shutdown

Все executable должны корректно обрабатывать:

```text
SIGTERM
SIGINT
```

Последовательность:

```text
stop accepting work
↓
finish current requests/tasks
↓
close HTTP server
↓
close Redis
↓
close PostgreSQL
```

Особенно важно для Railway deploy.

---

# 130. Security

Обязательные меры:

```text
Argon2id

refresh token rotation

rate limiting

request size limits

MIME validation

S3 presigned URLs

short upload expiration

Telegram webhook secret

SQL parameterization

CORS allowlist

secure cookies where applicable

audit-sensitive actions
```

---

# 131. Resume Security

CV содержит персональные данные.

Нельзя делать S3 bucket публичным.

Все объекты:

```text
private
```

Доступ:

```text
short-lived presigned URL
```

Object key:

```text
users/{userID}/resumes/{resumeID}/original.pdf
```

---

# 132. File Validation

Проверяем:

```text
maximum size
MIME
extension
actual file signature
```

Не доверяем:

```text
Content-Type
filename
```

из браузера.

---

# 133. Rate Limiting

Отдельные лимиты:

```text
login
register
password reset
resume upload
telegram linking
public webhook
```

---

# 134. Scheduler

Отдельный executable:

```text
cmd/scheduler
```

Он отвечает только за создание scheduled tasks.

Не выполняет тяжёлую работу самостоятельно.

```text
Scheduler
    ↓
enqueue SyncSource
    ↓
Worker
```

---

# 135. Worker Pools

Worker можно разделить логически:

```text
source
resume
matching
notification
```

На старте они могут находиться в одном binary.

Позже Railway:

```text
worker-source

worker-matching

worker-notification
```

без изменения domain/application layer.

---

# 136. Concurrency

Для source ingestion использовать bounded concurrency.

Не:

```go
for _, source := range sources {
	go sync(source)
}
```

без ограничений.

Использовать:

```text
worker pool
semaphore
rate limiter
```

---

# 137. Source Failure Isolation

Если:

```text
RemoteOK упал
```

это не должно останавливать:

```text
Greenhouse
Lever
Ashby
GitHub
```

Каждый source sync независим.

---

# 138. Circuit Breaker

Для нестабильных внешних сервисов можно добавить:

```text
circuit breaker
```

после появления реальной необходимости.

Не требуется в первой реализации.

---

# 139. Caching

Redis cache подходит для:

```text
skills dictionary
source configuration
rate limits
temporary Telegram tokens
short-lived queries
```

Но PostgreSQL остаётся canonical storage.

---

# 140. Search

MVP:

```text
PostgreSQL Full Text Search
+
pg_trgm
```

Не нужно сразу добавлять:

```text
Elasticsearch
OpenSearch
Meilisearch
```

Пока объём данных не требует этого.

---

# 141. Job Retention

Не удалять закрытые jobs сразу.

Хранить историю.

Например:

```text
active
↓
closed
↓
retain
```

Это полезно для:

```text
analytics
deduplication
company statistics
matching history
```

---

# 142. Source Discovery Pipeline

Полный flow:

```text
GitHub curated repository
        ↓
Discover Companies
        ↓
Normalize Company
        ↓
Find Careers URL
        ↓
Detect ATS
        ↓
Create Source
        ↓
Validate Source
        ↓
Enable Source
        ↓
Schedule Sync
        ↓
Fetch Jobs
```

---

# 143. Vacancy Pipeline

Полный production flow:

```text
Scheduler
    ↓
SyncSource
    ↓
Source Adapter
    ↓
ExternalJob[]
    ↓
RawJob persistence
    ↓
Normalizer
    ↓
Company Resolver
    ↓
Skill Extractor
    ↓
Location Classifier
    ↓
Deduplicator
    ↓
Job Repository
    ↓
JobCreated
    ↓
Outbox
    ↓
Candidate Retrieval
    ↓
Hard Filters
    ↓
Matching
    ↓
JobMatch
    ↓
Notification Decision
    ↓
Notification
    ↓
Telegram Adapter
    ↓
User
```

---

# 144. New Job Flow

```text
Greenhouse
↓
Senior Go Engineer
↓
RawJob stored
↓
normalize
↓
company = Acme
role = backend_engineer
seniority = senior
skills = Go, PostgreSQL, Kubernetes
remote = worldwide
employment = contract
↓
deduplicate
↓
new Job
↓
JobCreated
↓
find candidates
↓
user #123 → 91%
user #456 → rejected by country
user #789 → 74%
↓
user #123 minimum = 80%
↓
Notification created
↓
Telegram
```

---

# 145. Existing Job Update

Если salary/location/description изменились:

```text
External Job
↓
existing job found
↓
content hash differs
↓
normalize again
↓
update
↓
JobUpdated
```

Можно пересчитать affected matches.

---

# 146. Job Closed Flow

```text
Source Sync
↓
job missing repeatedly
↓
Job.Close()
↓
JobClosed
↓
active matches → expired
↓
pending notifications cancelled
```

---

# 147. User Profile Update

При изменении:

```text
skills
positions
country
employment preferences
salary
```

генерируем:

```text
ProfileChanged
```

После этого:

```text
enqueue MatchExistingJobs(userID)
```

Но не пересчитываем всю историю.

Например:

```text
active jobs
published < 30 days
```

---

# 148. Resume Update

```text
New Resume
↓
Parse
↓
Suggestions
↓
User accepts skills
↓
ProfileChanged
↓
Rematch recent jobs
```

---

# 149. Source Preference Update

Если пользователь отключает:

```text
RemoteOK
```

мы не удаляем вакансии RemoteOK из общей БД.

Меняется только пользовательский matching/feed.

---

# 150. Admin Logic

Нужна минимальная административная часть.

Admin может:

```text
view sources
enable/disable source
force sync
see sync failures
see ingestion statistics
see duplicate statistics
inspect raw job
inspect normalized job
retry failed operation
```

---

# 151. Audit Log

Для административных действий:

```text
audit_logs

actor_id
action
entity_type
entity_id
metadata
created_at
```

---

# 152. Error Model

Domain errors:

```go
var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
)
```

HTTP adapter переводит их:

```text
ErrNotFound       → 404
ErrAlreadyExists → 409
ErrForbidden      → 403
```

Domain не знает HTTP status codes.

---

# 153. API Error

Единый формат:

```json
{
  "error": {
    "code": "JOB_NOT_FOUND",
    "message": "job not found",
    "request_id": "..."
  }
}
```

---

# 154. Validation

Три уровня.

### Transport

```text
JSON format
required fields
length
```

### Application

```text
user allowed to perform operation?
resource exists?
```

### Domain

```text
business invariant valid?
```

---

# 155. Testing

Стратегия:

```text
Domain Unit Tests
Application Unit Tests
Repository Integration Tests
Adapter Contract Tests
API Integration Tests
End-to-End Tests
```

---

# 156. Domain Tests

Domain тестируется без:

```text
PostgreSQL
Redis
HTTP
Docker
```

Например:

```go
func TestJob_CannotCloseAlreadyClosedJob(t *testing.T)
```

---

# 157. Application Tests

Ports mock/fake:

```go
type FakeJobRepository struct {
	...
}
```

Проверяем use cases.

---

# 158. Repository Tests

Поднимаем настоящий:

```text
PostgreSQL
```

через testcontainers.

Проверяем реальные SQL queries.

---

# 159. Source Contract Tests

Каждый adapter должен проходить общий contract:

```text
Fetch returns ExternalJob
external ID exists
title exists
company resolvable
URLs valid
pagination terminates
```

---

# 160. Deployment

Railway:

```text
Project

├── api
│
├── worker
│
├── scheduler
│
├── telegram
│
├── postgres
│
├── redis
│
└── bucket
```

Можно использовать один Docker image с разными commands.

---

# 161. Docker

Один image:

```text
job-platform-backend
```

Railway commands:

```text
/api:
./app api

/worker:
./app worker

/scheduler:
./app scheduler

/telegram:
./app telegram
```

Либо отдельные binaries внутри image.

---

# 162. Migration Flow

Перед deployment:

```text
goose up
```

Migration должна быть совместима с rolling deployment.

Избегать сразу:

```text
DROP COLUMN
rename breaking columns
```

Использовать expand/contract migrations.

---

# 163. MVP Scope

Первая версия backend должна содержать:

```text
Auth

Profile

Skills

Preferences

Resume upload

Basic resume parsing

Telegram linking

Source Registry

Greenhouse adapter

Lever adapter

Ashby adapter

GitHub discovery adapter

Job normalization

Job persistence

Deduplication

Location eligibility

Rule-based matching

Telegram notifications

Feedback

Saved jobs

Source settings

Scheduler

Worker

Transactional outbox

Admin source monitoring
```

---

# 164. Не делать в MVP

Не нужны сразу:

```text
microservices

Kafka

Kubernetes

Elasticsearch

GraphQL

complex event sourcing

ML recommendation model

multiple AI providers

automatic job applications

browser automation for every site

100 source adapters

complex CQRS infrastructure
```

Это добавит сложность раньше, чем появится нагрузка.

---

# 165. Второй этап

После работающего MVP:

```text
pgvector

semantic matching

AI job analysis

AI resume analysis

feedback learning

email notifications

daily digest

company analytics

salary normalization

timezone compatibility

job recommendations

source auto-discovery

ATS auto-detection

additional ATS adapters
```

---

# 166. Основные архитектурные правила

### Rule 1

```text
Domain никогда не импортирует adapters.
```

### Rule 2

```text
Application зависит от ports, а не implementations.
```

### Rule 3

```text
PostgreSQL является source of truth.
```

### Rule 4

```text
Redis не хранит уникальное бизнес-состояние.
```

### Rule 5

```text
External sources всегда проходят через adapters.
```

### Rule 6

```text
Raw external data сохраняется до normalization.
```

### Rule 7

```text
Все background operations idempotent.
```

### Rule 8

```text
Telegram является notification adapter, а не частью matching domain.
```

### Rule 9

```text
S3 является port, а не конкретным vendor.
```

### Rule 10

```text
AI является дополнительным adapter.
Core matching должен работать без AI.
```

### Rule 11

```text
Не считать remote автоматически worldwide.
```

### Rule 12

```text
Не отправлять вакансию до проверки country eligibility.
```

### Rule 13

```text
Одна vacancy может иметь несколько external sources.
```

### Rule 14

```text
Official source имеет приоритет над aggregator.
```

### Rule 15

```text
Matching и ingestion должны масштабироваться независимо.
```

---

# 167. Итоговая схема

```text
                         INTERNET
                            │
        ┌───────────────────┼────────────────────┐
        │                   │                    │
        ↓                   ↓                    ↓
   Greenhouse             Lever                Ashby
        │                   │                    │
        └───────────────────┼────────────────────┘
                            ↓
                     Source Adapters
                            │
                            ↓
                         Raw Jobs
                            │
                            ↓
                       Normalizer
                            │
             ┌──────────────┼───────────────┐
             ↓              ↓               ↓
          Skills         Location        Company
             │              │               │
             └──────────────┼───────────────┘
                            ↓
                       Deduplicator
                            │
                            ↓
                         Jobs DB
                            │
                            ↓
                       JobCreated
                            │
                            ↓
                          Outbox
                            │
                            ↓
                         Worker
                            │
                            ↓
                    Candidate Retrieval
                            │
                            ↓
                       Hard Filters
                            │
                            ↓
                         Matching
                            │
                            ↓
                        Job Matches
                            │
                            ↓
                    Notification Rules
                            │
                            ↓
                       Notifications
                            │
                            ↓
                    Telegram Adapter
                            │
                            ↓
                           USER


USER
 │
 ├── Auth
 │
 ├── Profile
 │    ├── Skills
 │    ├── Positions
 │    ├── Location
 │    └── Preferences
 │
 ├── Resume
 │    ↓
 │  S3 Storage
 │    ↓
 │  Resume Worker
 │    ↓
 │  Suggestions
 │
 ├── Sources
 │    └── Enable / Disable
 │
 └── Telegram
      ↓
   Deep Link
      ↓
   One-Time Token
      ↓
   Telegram Account
      ↓
   Notifications
```

# 168. Главный backend pipeline

```text
DISCOVER
    ↓
FETCH
    ↓
STORE RAW
    ↓
NORMALIZE
    ↓
CLASSIFY
    ↓
DEDUPLICATE
    ↓
PERSIST
    ↓
MATCH
    ↓
FILTER
    ↓
RANK
    ↓
NOTIFY
    ↓
COLLECT FEEDBACK
    ↓
IMPROVE MATCHING
```

Это является базовой backend-архитектурой проекта на **Go + PostgreSQL + Redis + S3 + Telegram с Hexagonal Architecture**.
