# AI Workflow Automation Engine

A backend engine that converts natural language workflow descriptions into structured, executable workflows using AI. Built with Go, PostgreSQL, and Groq AI.

## How It Works

1. User describes a workflow in plain English — *"Check if age is above 18, if not reject, if yes send a welcome email"*
2. AI (Groq — Llama 3.3) converts it into a structured DSL
3. DSL is stored in PostgreSQL as JSONB
4. Execution engine runs the workflow against any input data
5. Returns a step-by-step result — approved or rejected
6. Every execution is logged with full audit trail

## Tech Stack

- **Go** — backend language
- **Gin** — HTTP framework
- **PostgreSQL** — database (JSONB for DSL storage)
- **GORM** — ORM
- **Groq AI** (Llama 3.3 70B) — natural language to DSL conversion
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
│   └── workflow_handler.go        # HTTP layer — parses requests, writes responses
├── services/
│   ├── workflow_service.go        # business logic
│   ├── ai_services.go             # Groq AI integration
│   └── execution_engine.go       # runs DSL steps against input data
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
├── Dockerfile
└── docker-compose.yml
```

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
```

3. Start the application
```bash
docker compose up --build
```

The server starts at `http://localhost:8080`

Open `http://localhost:8080` in your browser to use the UI.

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

## Database

Two tables:

- **workflows** — stores name, description, DSL (jsonb) and current status
- **execution_logs** — stores every execution run with input data, step results, status and timestamp