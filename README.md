# AI Workflow Automation Engine

A backend engine that converts natural language workflow descriptions into structured, executable workflows using AI. Built with Go, PostgreSQL, and Groq AI.

## How It Works

1. User describes a workflow in plain English — *"Check if age is above 18, if not reject, if yes send a welcome email"*
2. AI (Groq — GPT-OSS 120B) converts it into a structured DSL
3. The generated DSL is validated — if it's invalid, it's sent back to the AI once with the validation errors for self-correction
4. DSL is stored in PostgreSQL as JSONB
5. Execution engine runs the workflow against any input data
6. Returns a step-by-step result — approved or rejected
7. Every execution is logged with full audit trail

## Tech Stack

- **Go** — backend language
- **Gin** — HTTP framework
- **PostgreSQL** — database (JSONB for DSL storage)
- **GORM** — ORM
- **Groq AI** (GPT-OSS 120B) — natural language to DSL conversion
- **Docker** — containerization
- **Vanilla JS** — frontend UI

## Project Structure

```
workflow-engine/
├── main.go                        # entry point, wires all layers
├── config/
│   ├── db.go                      # database connection
│   └── env.go                     # environment variable loader
├── handlers/
│   ├── workflow_handler.go        # HTTP layer — parses requests, writes responses
│   └── job_handler.go             # async job status lookup
├── services/
│   ├── workflow_service.go        # business logic
│   ├── ai_services.go             # Groq AI integration
│   ├── dsl_validator.go           # validates AI-generated DSL
│   ├── dsl_pipeline.go            # generate -> validate -> retry, with timing
│   └── execution_engine.go       # runs DSL steps against input data
├── worker/                        # async execution worker pool (see below)
│   ├── job.go                     # Job, JobStatus, concurrency-safe Registry
│   ├── pool.go                    # fixed worker pool over a bounded channel
│   └── dispatcher.go              # Enqueue/Get — the dependency handlers hold
├── logger/                        # structured logging + metrics (see Observability)
│   ├── logger.go                  # slog JSON setup, LOG_LEVEL
│   ├── metrics.go                 # atomic Counters, global Metrics singleton
│   └── id.go                      # shared execution/job ID generator
├── repos/
│   ├── workflow_repo.go           # workflow database queries
│   └── execution_log_repo.go     # execution history queries
├── models/
│   ├── workflows.go               # Workflow DB model
│   ├── dsl.go                     # DSL structure (steps, conditions, actions)
│   ├── execution.go               # execution result types
│   └── execution_log.go          # execution history DB model
├── routes/
│   └── routes.go                  # route registration
├── static/
│   ├── index.html                 # frontend UI
│   ├── style.css                  # styles
│   └── app.js                     # API calls and interactivity
├── eval/                          # LLM evaluation harness (see below)
│   ├── dataset.json               # baseline test cases
│   ├── dataset_adversarial.json   # edge-case test cases
│   ├── types.go, io.go, runner.go, report.go
│   └── cmd/main.go                # CLI entrypoint
├── tests/
│   └── integration/                # build-tag gated HTTP + DB tests (see Testing)
├── Dockerfile
└── docker-compose.yml
```

Unit tests live alongside the code they test (e.g. `services/dsl_validator_test.go`) rather than in a separate tree — see **Testing** below.

## DSL Structure

The AI generates a structured DSL with two step types:

**Condition** — evaluates a field from the input data
```json
{
  "type": "condition",
  "field": "age",
  "operator": "gt",
  "value": 18,
  "on_pass": "next",
  "on_fail": "reject"
}
```

**Action** — performs an operation
```json
{
  "type": "action",
  "action": "send_email",
  "params": { "template": "welcome" }
}
```

Supported operators: `gt`, `lt`, `eq`, `gte`, `lte`

Supported actions: `approve`, `reject`, `send_email`, `assign_role`, `webhook`

## DSL Validation and Self-Correction

AI-generated output isn't guaranteed to match the schema, so every DSL is validated before it's saved:

- step type must be `condition` or `action`
- condition steps require `field`, a valid `operator`, `value`, `on_pass` and `on_fail`
- comparison operators (`gt`, `lt`, `gte`, `lte`) require a numeric `value`
- `on_pass` / `on_fail` must be `next` or `reject`
- action steps require a valid `action` name and any params it needs (`send_email` → `template`, `assign_role` → `role`, `webhook` → `url`)
- webhook URLs are checked and normalized — a bare domain like `www.example.com` is accepted and defaults to `https://`

If validation fails, the invalid JSON and the specific validation errors are sent back to the AI once, asking it to self-correct. The corrected response is validated again — if it still fails, `generate-dsl` returns an error instead of saving a broken workflow.

Validation happens at **generate time only** — `execute` reads whatever DSL is stored and runs it without re-validating. This is safe today because `generate-dsl` is the only code path that ever writes `workflow.DSL` (`update-workflow` only touches name/description), but it's an invariant maintained by that being true, not a check enforced at execution time. If a future change adds another way to write a workflow's DSL, or a validation rule is tightened later, this section needs revisiting.

## Evaluation Harness

A standalone harness in `eval/` runs a labeled dataset of workflow descriptions through the real generate → validate → retry pipeline and the execution engine, then reports aggregate metrics. It runs in-process (no HTTP, no Postgres writes) so it's fast and doesn't pollute the database with test workflows.

Run it with:
```bash
go run ./eval/cmd                                            # baseline dataset (40 cases)
go run ./eval/cmd --dataset eval/dataset_adversarial.json     # harder edge cases (8 cases)
go run ./eval/cmd --dataset eval/dataset.json,eval/dataset_adversarial.json  # both combined
```

Each test case gives a description plus one or more sample inputs with an expected outcome (status + which actions should fire). The generated DSL is graded **behaviorally** — executed against each sample input and compared to the expected outcome — rather than compared field-by-field against a fixed DSL, since multiple differently-shaped DSLs can be equally correct.

The report covers:
- **Valid DSL rate** — did the final DSL (after an optional retry) pass validation
- **Semantic correctness** — did executing it produce the expected status and actions
- **Initial generation pass rate** and **retry success rate** — how often the first attempt was valid, and how often the self-correction retry fixed a bad one
- **Executed-without-error rate** — of validated DSLs, how many ran cleanly through the execution engine (a separate signal from correctness — catches gaps where validation passes but execution still breaks)
- **Average latency** per stage (LLM call, validation, retry) — with time spent waiting out a Groq rate limit (429) tracked and reported separately, so it never silently inflates the call-latency numbers
- A per-category breakdown

Results are written to `eval/results/<timestamp>/` (gitignored) as `cases.json` (full per-case detail) and `report.md` (the rendered summary).

## Testing

Unit tests live next to the code they test and have no external dependencies:
```bash
go test ./...
```
Covers DSL validation, the execution engine (operators, short-circuit behavior, webhook URL normalization via `httptest`), the generate→validate→retry pipeline (via a fake AI client — no real Groq calls), rate-limit backoff, `.env` loading, the worker pool (happy path, failure path, panic recovery, queue-full rejection — via a fake `Executor`, no real DB or AI), and the metrics counters (concurrent-increment correctness via goroutines, since they're backed by `atomic.Int64`).

A separate integration suite exercises the real HTTP API against a real Postgres (the one `docker compose up db` starts), seeding DSLs directly into the database rather than calling `generate-dsl` — testing AI output quality is the evaluation harness's job, not this suite's:
```bash
go test -tags=integration -count=1 ./tests/integration/...
```
It's gated behind the `integration` build tag so a plain `go test ./...` never requires a database. `-count=1` bypasses Go's test cache — safe to skip for the unit tests above since they're pure logic, but worth keeping here since Go's cache only tracks source files, not live database state: if you change something in Postgres directly (not through a `.go` file), a cached result would otherwise still report a stale "ok". Each test truncates `workflows` and `execution_logs` first (for isolation), and the suite truncates both again when it finishes, so run this against a local dev database, not one with data you want to keep.

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Groq API key — [console.groq.com](https://console.groq.com)

### Setup

1. Clone the repository
```bash
git clone https://github.com/raghuvanshichandrapratap-art/workflow-engine
cd workflow-engine
```

2. Create a `.env` file in the project root
```env
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=workflow_engine
DB_PORT=5432
GROQ_API_KEY=your_groq_api_key
PORT=5050
WORKER_COUNT=4
JOB_QUEUE_CAPACITY=50
LOG_LEVEL=info
```

3. Start the application
```bash
docker compose up --build
```

The server starts at `http://localhost:5050` (override with `PORT` in `.env` if that's also taken).

Open `http://localhost:5050` in your browser to use the UI.

## API Endpoints

### Create a Workflow
```
POST /workflow
```
```json
{
  "name": "Employee Onboarding",
  "description": "Check if age is above 18, if not reject. If yes send welcome email and assign engineer role."
}
```

### Generate DSL from Description
```
POST /workflow/:id/generate-dsl
```
Sends the workflow description to Groq AI and saves the generated DSL back to the workflow.

### Execute a Workflow
```
POST /workflow/:id/execute
```
```json
{
  "name": "John Doe",
  "age": 25,
  "email": "john@company.com",
  "documents_submitted": true,
  "role": "engineer"
}
```

Response:
```json
{
  "status": "approved",
  "steps": [
    { "type": "condition", "passed": true, "message": "age(25) > 18: true" },
    { "type": "action", "action": "send_email", "message": "email sent with template 'welcome'" },
    { "type": "action", "action": "assign_role", "message": "role 'engineer' assigned" }
  ]
}
```

### Execute a Workflow Asynchronously
```
POST /workflow/:id/execute-async
```
Same request body as `/execute`. Instead of blocking until the workflow finishes, this enqueues a job and returns immediately:
```json
{ "job_id": "a1b2c3d4e5f6a7b8", "status": "queued" }
```
Returns `503` instead if the job queue is full (see **Async Execution** below).

### Check Job Status
```
GET /jobs/:job_id
```
```json
{
  "id": "a1b2c3d4e5f6a7b8",
  "workflow_id": "1",
  "status": "completed",
  "result": { "status": "approved", "steps": [...] },
  "created_at": "...",
  "started_at": "...",
  "finished_at": "..."
}
```
`status` is one of `queued`, `running`, `completed`, `failed` (with an `error` field instead of `result` when failed). A completed or failed job is also recorded in `execution_logs`, identically to a synchronous `/execute` call — it shows up in the history endpoint below too.

### Get Execution History
```
GET /workflow/:id/executions?page=1&limit=10
```
Returns paginated execution logs for a workflow — each entry includes status, input data, step results and timestamp.

```json
{
  "data": [...],
  "page": 1,
  "limit": 10,
  "total": 24,
  "total_pages": 3
}
```

### Other Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/workflow/:id` | Get a workflow |
| PUT | `/workflow/:id` | Update a workflow |
| DELETE | `/workflow/:id` | Delete a workflow |
| GET | `/metrics` | Execution counters — see **Observability** |

## Architecture

The project follows a layered architecture where each layer only communicates with the one directly below it:

```
HTTP Request
     ↓
Handler       — parses and validates request, writes response
     ↓
Service       — business logic, orchestrates AI, engine and DB
     ↓
Repository    — database queries
     ↓
PostgreSQL
```

The AI service and execution engine are injected into the service layer as dependencies, keeping each layer independently testable and replaceable.

## Async Execution (Worker Pool)

`/execute-async` runs workflows through a fixed-size pool of goroutines instead of on the HTTP request's own goroutine:

```
POST /execute-async
     ↓
Dispatcher.Enqueue — creates a Job (status: queued), submits it to a buffered channel
     ↓                                     (non-blocking — if the channel is full, the
     ↓                                      job is marked failed and 503 is returned)
Job queue (buffered chan *Job, capacity JOB_QUEUE_CAPACITY)
     ↓
WORKER_COUNT worker goroutines, each: for job := range queue { process(job) }
     ↓
process: mark running → call the same WorkflowService.ExecuteWorkflow the
         synchronous endpoint uses → mark completed or failed
```

A few things worth calling out if you're reading this to learn the pattern, not just use it:

- **The worker package never touches the DB or AI directly.** It depends only on a small `Executor` interface (`ExecuteWorkflow(workflowID, input) (*ExecutionResult, error)`), which `*services.WorkflowService` already satisfies. Since that method already writes to `execution_logs`, a job run through the pool produces history identical to a synchronous call — nothing extra to wire up.
- **Status updates are concurrency-safe.** `Registry` guards its map with one mutex; every transition (`MarkRunning`, `MarkCompleted`, `MarkFailed`) goes through a method that takes the lock, and `Get` returns a copy rather than the internal pointer — so an HTTP handler goroutine polling status can never race with a worker goroutine mid-update.
- **The queue is bounded, and submission never blocks.** `Submit` uses a `select` with a `default` case — if the channel's buffer is full, it returns `false` immediately instead of making the HTTP request wait for space. This is what turns an overloaded worker pool into a fast `503` instead of a slowly-growing pile of stuck HTTP connections.
- **One job panicking doesn't take down a worker permanently.** `process` recovers from a panic and marks that one job failed — the worker goroutine keeps running afterward and picks up the next job.
- **Shutdown is sequenced, not immediate.** On SIGINT/SIGTERM, `main.go` stops the HTTP server first (so no new jobs can be submitted), *then* closes the job channel and waits for already-queued jobs to finish — rather than dropping in-flight work on the floor.

## Observability

Structured JSON logs (via `log/slog`, level controlled by `LOG_LEVEL`) and a handful of counters, covering the execution lifecycle end to end.

**Logs** use two separate correlation IDs, because they track two genuinely different things:

- **`execution_id`** — generated fresh inside `WorkflowService.ExecuteWorkflow` every time it's called. Since *both* the synchronous `/execute` handler and the async worker pool's `process` call this one method, every execution gets `"execution started"` / `"execution completed"` or `"execution failed"` log lines with `execution_id`, `workflow_id`, `duration_ms`, and (on failure) `error` — for free, on both paths, with no duplicated logging logic.
- **`job_id`** — only exists for the async path, logged by the `worker` package itself (`"job queued"`, `"job running"`, `"job completed"`/`"job failed"`, or `"job queue full"` if the queue was rejected before an execution ever started). This is a different layer from `execution_id` on purpose — a queued job and a running execution aren't the same thing, and forcing them to share one ID would mean changing the `Executor` interface for no real benefit at this scale. The two are correlated by `workflow_id` and near-identical timestamps instead.

Example log line:
```json
{"time":"...","level":"INFO","msg":"execution completed","execution_id":"a1b2c3d4e5f6a7b8","workflow_id":"9","duration_ms":12,"status":"approved"}
```

**Counters** (`GET /metrics`, plain JSON, backed by `atomic.Int64`):
```json
{ "executions_started": 12, "executions_succeeded": 10, "executions_failed": 2, "queue_errors": 0 }
```
`executions_failed` covers every way an execution attempt can fail — workflow not found, no DSL generated yet, a bad DSL step, whatever — they're all failed execution attempts from the caller's perspective. `queue_errors` is deliberately separate: it's the one failure class that happens *before* `ExecuteWorkflow` is ever reached (the job queue was full), so it's never double-counted in `executions_failed`.

## Database

Two tables:

- **workflows** — stores name, description, DSL (jsonb) and current status
- **execution_logs** — stores every execution run with input data, step results, status and timestamp