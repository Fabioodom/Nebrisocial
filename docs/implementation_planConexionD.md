# Implementation Plan: Search and Lazy-loaded Sidebars (Phase D)

This plan details the steps required to implement dynamic node search and lazy-loaded sidebar components via HTMX.

## User Review Required

> [!IMPORTANT]
> - A new file `internal/handlers/views/components.templ` will be created to house the `@LeftSidebarContent` and `@RightSidebarContent` components.
> - The global `Layout` in `layout.templ` will be updated to fetch left and right sidebars dynamically using `hx-get="/components/sidebar/left"` and `hx-get="/components/sidebar/right"` on page load (`hx-trigger="load"`). This decouples sidebar loading from initial page loads and enables lazy loading.
> - The Search input in the navbar will target `#main-feed` to dynamically reload the list of `@NodeCard` matching the search string.

---

## Proposed Changes

### Database Layer

#### [MODIFY] [node_repo.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/platform/database/node_repo.go)
- Implement `SearchNodes(db *sql.DB, queryStr string, userID *string) ([]Node, error)`:
  - Searches nodes where title or description matches `ILIKE '%' || $1 || '%'`.
  - Populates like count, liked, and saved states similarly to `ListNodes`.
- Implement `GetPopularNodes(db *sql.DB, limit int) ([]Node, error)`:
  - Queries nodes ordered by their total likes count descending (`ORDER BY likes_count DESC LIMIT $1`).
  - Includes a subquery to compute the likes count dynamically.
- Implement `GetCategoriesWithNodes(db *sql.DB) (map[string][]Node, error)`:
  - Queries unique active categories and their nodes, then groups them in Go as `map[string][]Node`.

---

### Backend Controllers & Routing

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Create `SearchHandler(db *sql.DB)`:
  - Handles `GET /search`.
  - Reads `q` parameter. If empty, calls `database.ListNodes(db, userID)`. Otherwise calls `database.SearchNodes(db, q, userID)`.
  - Render only the `@views.NodeGrid(nodes)` or a list of `@views.NodeCard(n)` depending on target. Rendering a list of `@views.NodeCard(n)` loop is cleanest for `#main-feed` swapping.
- Create `LeftSidebarHandler(db *sql.DB)`:
  - Handles `GET /components/sidebar/left`.
  - Calls `database.GetCategoriesWithNodes(db)` and renders `@views.LeftSidebarContent(categories)`.
- Create `RightSidebarHandler(db *sql.DB)`:
  - Handles `GET /components/sidebar/right`.
  - Calls `database.GetPopularNodes(db, 3)`.
  - Uses `regexp` to find all `#tags` (words starting with `#`) in the popular nodes' descriptions.
  - Groups and deduplicates the hashtags to return a `trends []string` slice.
  - Renders `@views.RightSidebarContent(popularNodes, trends)`.

#### [MODIFY] [main.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/cmd/nodal/main.go)
- Register endpoints:
  - `GET /search` -> `handlers.SearchHandler(db)`
  - `GET /components/sidebar/left` -> `handlers.LeftSidebarHandler(db)`
  - `GET /components/sidebar/right` -> `handlers.RightSidebarHandler(db)`

---

### Frontend Layout & Views

#### [MODIFY] [layout.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/layout.templ)
- Update Search input in `NavBar` to include:
  - `name="q" hx-get="/search" hx-target="#main-feed" hx-trigger="keyup changed delay:500ms, search"`
- Update `Sidebar()` to render an empty div that triggers dynamic load:
  - `<div hx-get="/components/sidebar/left" hx-trigger="load"></div>`
- Update `Widgets()` to render an empty div that triggers dynamic load:
  - `<div hx-get="/components/sidebar/right" hx-trigger="load"></div>`

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- Add `id="main-feed"` to the feed loop wrapper or `NodeGrid` wrapper so that search swap targets it cleanly.

#### [NEW] [components.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/components.templ)
- Create components:
  - `LeftSidebarContent(categories map[string][]database.Node)`: Loop over categories to render grouped links (e.g. Manga & Anime, Gaming, etc.).
  - `RightSidebarContent(popularNodes []database.Node, trends []string)`: Render popular top-3 nodes and hashtags badges.

---

## Verification Plan

### Automated Tests
- Run `templ generate` to compile all new components and views.
- Run `go build ./...` to verify Go type safety and handlers compilation.

### Manual Verification
- Verify that searching in the navbar filter updates the main feed seamlessly.
- Verify that the left navigation menu and right widgets load dynamically in the browser via background HTTP calls.
- Inspect network requests using Developer Tools to ensure `/components/sidebar/left` and `/components/sidebar/right` load correctly once per page load.
