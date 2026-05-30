# Implementation Plan: Trending Topics Engine & Explore Filter

This plan describes the transformation of the right sidebar widget from "Popular Nodes" to a pure "Trending Topics" (hashtags frequency engine), and the implementation of search filtering in the Explore view.

## User Review Required

> [!IMPORTANT]
> - The `@RightSidebarContent` template in `components.templ` will be refactored to accept only `trends []handlers.Trend` (or `views.Trend` depending on how we expose the struct).
> - `RightSidebarHandler` will extract all titles and descriptions of recent nodes, run a regex to identify hashtags, count their frequencies, sort them, and select the top 5.
> - The `ExploreHandler` (`GET /explore`) will check for a `q` parameter, calling `SearchNodes` if present, to filter exploration nodes.

## Proposed Changes

### Backend Controllers

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Define `Trend` struct:
  ```go
  type Trend struct {
      Name  string
      Count int
  }
  ```
- Update `RightSidebarHandler`:
  - Query all nodes using `database.ListNodes(db, nil)`.
  - Extract hashtags from `Title` and `Description` using `regexp.MustCompile(`#[a-zA-Z0-9_]+`)`.
  - Keep a frequency map, sort results, pick the top 5, and pass them to `@views.RightSidebarContent(trends)`.

#### [MODIFY] [home.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/home.go)
- In `ExploreHandler`:
  - Retrieve the `q` parameter from `r.URL.Query().Get("q")`.
  - If `q` is not empty, query nodes using `database.SearchNodes(db, q, userID)` instead of `ListNodes`.

---

### Frontend Views & Templates

#### [MODIFY] [components.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/components.templ)
- Define `Trend` struct matching backend or define inside the views package. Defining a custom struct in database or views is clean. Let's define `Trend` in `database` or `views` so both template and handler can use it. Defining it in `database` packages is very easy:
  ```go
  type Trend struct {
      Name  string
      Count int
  }
  ```
  in `internal/platform/database/node_repo.go`.
- Redesign `@RightSidebarContent(trends []database.Trend)`:
  - Remove "Nodos Populares" completely.
  - Render a clean and elegant "Tendencias" list.
  - Each item renders the hashtag `Name` and subtitle `"{Count} nodos activos"`.
  - Wrap the whole item in an `<a>` link pointing to `/explore?q={NameWithoutHash}`.

---

## Verification Plan

### Automated Tests
- Run `templ generate` to compile templates.
- Run `go build ./...` to verify package compilation.
