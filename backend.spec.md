# TwentyFive Backend Specification

## Overview
- Implement a lightweight Go HTTP service that serves the existing `index.html` SPA and exposes a JSON API that mirrors the Alpine.js state.
- Persist board state (categories, tasks, backburner, archives, metadata) to a JSON file on disk; load it at startup and flush changes atomically.
- Design for a future single-page app front end that continues to manage rich UI interactions while delegating CRUD operations and persistence to Go.

## Goals
- Serve the static assets (`index.html`, CSS/JS bundles, static media) from Go without requiring an external web server.
- Provide REST-ish JSON endpoints that Alpine can call via `fetch` for:
  - Fetching the entire board state.
  - Creating, updating, deleting tasks.
  - Moving tasks between categories/backburner/archive.
  - Renaming categories.
- Ensure concurrent client requests do not corrupt the persisted state.
- Keep latency low and implementation small; prioritize clarity over extensive infrastructure.

## Non-Goals
- Multi-user synchronization or authentication (single-user desktop/server mode only).
- Real-time websockets or push updates (polling or on-demand fetch suffices for now).
- Full-blown relational schema or migrations (flat JSON persistence only).

## High-Level Architecture
- `main.go` bootstraps:
  - Configuration (port, data file path) via flags/env.
  - Dependency container holding the in-memory board store and persistence layer.
  - HTTP router (standard library `net/http` + `http.ServeMux` or `chi` if preferred).
- `Store` component:
  - Loads JSON from disk into an in-memory `BoardState`.
  - Provides thread-safe methods (`sync.RWMutex`) for read/write operations.
  - On write, updates memory and flushes to disk using temp file + rename to avoid partial writes.
- `Handlers` translate HTTP requests to store operations and respond with JSON.
- Static asset handler serves files from `./public` (containing `index.html` and bundled assets).

## Data Model
```go
type BoardState struct {
    Categories []Category `json:"categories"`
    Backburner []Task     `json:"backburner"`
    Archives   []Task     `json:"archives"`
}

type Category struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Tasks []Task `json:"tasks"`
}

type Task struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Description string        `json:"description"`
    Notes       string        `json:"notes"`
    State       string        `json:"state"` // todo | doing | blocked | delegated | done
    Size        int           `json:"size"`
    Links       []TaskLink    `json:"links"`
    Urgent      bool          `json:"urgent"`
    Focused     bool          `json:"focused"`
    SourceID    string        `json:"sourceId,omitempty"`
    Source      string        `json:"source,omitempty"`
}

type TaskLink struct {
    Text string `json:"text"`
    URL  string `json:"url"`
}
```
- IDs remain opaque strings (front end currently uses `uid()`); backend should accept provided IDs and generate new ones (e.g., UUIDv4) when creating tasks/categories server-side.

## HTTP API
All endpoints accept/return JSON and live under `/api`.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/board` | Fetch entire `BoardState`. |
| POST | `/api/tasks` | Create a task. Request includes category ID (or `"backburner"`/`"archive"`). Response returns updated task. |
| PATCH | `/api/tasks/{id}` | Update task fields (name, description, notes, state, size, links, urgent, focused). Accepts partial payload. |
| POST | `/api/tasks/{id}/move` | Move task to destination (`categoryId`, `backburner`, `archive`) with optional index ordering. |
| DELETE | `/api/tasks/{id}` | Delete task from archive permanently. |
| POST | `/api/categories` | Create a new category (if we ever allow this). |
| PATCH | `/api/categories/{id}` | Rename category or reorder tasks array. |
| POST | `/api/board/focus` | Toggle global focus task (ensures only one focused). |

- Responses:
  - On success: `200 OK` (or `201 Created` for POST) with JSON body.
  - On validation errors: `400` with `{ "error": "message" }`.
  - On missing resources: `404`.
  - On persistence failures: `500`.

## Persistence Strategy
- Data file path default: `data/board.json`.
- Loading:
  - If file missing, initialize with seed data (mirroring current demo state).
  - Validate JSON schema (ensure categories/backburner/archive arrays exist).
- Saving:
  - Write to temporary file (e.g., `board.json.tmp`), then `os.Rename`.
  - Use `fsync` (`File.Sync()`) before rename to ensure durability.
- Consider periodic auto-save (debounce writes) but start with immediate flush per mutation.

## Concurrency & Transactions
- Wrap `BoardState` inside a struct with `sync.RWMutex`.
- All read handlers acquire `RLock`; write handlers use `Lock`.
- Complex operations (move task) execute entirely inside the lock to keep state consistent.

## Validation Rules
- Enforce column capacity: sum of `size` in category ≤ 5; if over capacity on create/move, return `409 Conflict` with suggested backburner fallback.
- Validate `state` is one of allowed values; `size` in [1,5].
- Ensure only one `urgent` per category and one `focused` overall; server should normalize these flags on updates.

## Static Asset Serving
- Serve `index.html` at `/` with `http.FileServer`.
- All other static assets (CSS, JS bundles, images) placed in `./public` and served under `/`.
- SPA fallback: requests for unknown paths return `index.html`.

## Error Handling & Logging
- Centralize error responses with helper returning JSON and status code.
- Use structured logging (e.g., `log/slog`) for request errors, persistence failures.
- Return safe messages to client; log detailed stack traces server-side.

## Testing Strategy
- Unit tests for store operations (capacity checks, move logic, persistence).
- Integration tests using `httptest.Server` to cover API flows.
- Include fixture data for sample board state.

## Deployment Notes
- Binary can run locally (`go run .`) or containerized.
- Provide sample `docker-compose.yml` with volume mounting `data/`.
- Document environment variables: `PORT`, `DATA_FILE`, `ASSET_DIR`.

## Future Enhancements
- Add user authentication/token once multi-user support required.
- Introduce optimistic concurrency (ETags) to handle multiple tabs.
- Replace polling with WebSockets for real-time sync.
- Support import/export endpoints for backups.

## Open Questions
- Should server or client be source of truth for ID generation going forward?
- Do we support ordering of categories/tasks beyond existing layout?
- Expected frequency of writes (to tune debounce/backoff)?
- Persistence encryption or locking when multiple processes run?
