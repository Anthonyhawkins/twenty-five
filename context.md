# TwentyFive Project Overview

## What Is TwentyFive?
- TwentyFive is a client-side productivity board that blends kanban, time blocking, and point-based prioritization.
- The UI presents a five-column grid (each colmn a category of tasks) with a hard cap of five “points” per column.
- Tasks carry metadata—size (1–5), description, notes, links, state (todo/doing/done), urgent and focus flags—to help users balance workload visually.
- Overflow tasks land in a Backburner list, while archived cards remain accessible in an Archive section.

## Current Front-End
- Built with Alpine.js for stateful interactivity and TailwindCSS for visual styling inside `index.html`.
- All state currently lives in the Alpine component (browser memory only); there is no persistence across reloads.
- Features include:
  - Add/Edit task modals with support for notes, size controls, and multi-line link definitions.
  - Inline category renaming.
  - State cycling via card click, single focused task, single urgent task per column.
  - Backburner routing when column capacity is exceeded; explicit archive/restore/delete flows.

## Near-Term Goals
- Preserve the polished front-end experience while introducing a Go backend to:
  - Serve the SPA assets.
  - Expose REST-style endpoints enabling the Alpine app to load/save state.
  - Persist board data to a JSON file safely.
- Maintain the 25-point constraint logic server-side to ensure consistency.

## Longer-Term Aspirations
- Extend the platform with features such as drag-and-drop reordering, shareable boards, localStorage caching, and optional calendar/focus modes.
- Support richer collaboration scenarios by layering on authentication and multi-user synchronization in future iterations.

## Guiding Principles
- Keep the UX compact, responsive, and minimal; avoid clutter that disrupts the grid metaphor.
- Opt for clarity and maintainability in both front- and back-end code; prefer small, composable components.
- Treat persistence and networking as transparent enhancements—front-end behavior should remain smooth even while transitioning to server-backed state.

