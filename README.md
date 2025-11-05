# TwentyFive

TwentyFive is a personal task organizer designed to run locally for a single user. It combines lightweight kanban columns with fixed capacity (five "points" per column) so you can visually balance work, backlog, and completed items.

## Features

- Five-column board with 25 total capacity points (5 per column)
- Tasks carry metadata: size (1–5), state (`todo`, `doing`, `blocked`, `delegated`, `done`), notes, private links, focus + urgent flags
- Backburner for overflow tasks and Archive for completed/removed cards
- Inline category renaming, quick add/edit modals, and urgency/focus controls
- Go backend persists board state to JSON and serves the single-page UI

## Prerequisites

- Go 1.22+

## Running the App

```sh
# install dependencies (none beyond Go standard library)
# run the server
GOCACHE=$(pwd)/.gocache go run ./cmd/server
```

The server listens on `http://localhost:8080` by default. Open that address in your browser to use the board. All data is saved to `data/board.json` in the project root.

## Project Structure

```
cmd/server        # Go entry point
internal/app      # server logic, persistence, HTTP handlers
internal/assets   # embedded SPA HTML
context.md        # project overview & goals
```

## Notes

- The board is meant for one person running locally; no authentication or multi-user features exist.
- The front end relies on Alpine.js and Tailwind via CDN (embedded HTML); everything runs client-side once served.
- Archived/backburner tasks remember their original category even if columns are renamed.

## Future Enhancements

See `context.md` for roadmap ideas such as drag-and-drop ordering, localStorage caching, and shareable boards.
