# Reddit/Discord Style UI/UX Enhancements for Nodal Frontend

We want to elevate the visual aesthetic of the Nodal frontend, bringing it to the visual polish of platforms like Reddit and Discord. The changes span the Sidebar (grouped categories & inline icons), the central feed (collapsible create node box and gradient welcome card), the Node Card component (refined borders, premium hover states, and color-coded category badges), and the search bar in the NavBar (darker theme, modern SVG search icon, and visual keyboard shortcut).

## User Review Required

> [!IMPORTANT]
> The database implementation of `CreateNode` currently does not persist the node `category` field (it is omitted in the query). We plan to modify `internal/platform/database/node_repo.go` to accept and persist this column to allow category badges to render properly for newly created nodes.

> [!NOTE]
> Since Nodal does not run a Tailwind CSS compiler synchronously, we will implement these improvements by:
> 1. Updating the specific component styling in `static/css/components.css`.
> 2. Providing standard CSS utility declarations (e.g., `.hover\:bg-bg-surface-hover`, `.transition-colors`, etc.) inside `static/css/components.css` to fully support utility classes used in the Templ structures.

## Open Questions
There are no major open questions, but we will configure the default category on node creation to be "General" if none is selected, matching standard behavior.

---

## Proposed Changes

### Database Layer
We will update the database repository to support persisting categories.

#### [MODIFY] [node_repo.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/platform/database/node_repo.go)
- Modify `CreateNode` signature to `CreateNode(db *sql.DB, title, description, category string) (string, error)`.
- Update the SQL query in `CreateNode` to insert the `category` column: `INSERT INTO nodes (slug, title, description, category) VALUES ($1, $2, $3, $4) RETURNING id`.

---

### Handlers & Templates
We will update the HTTP handlers to pass the category from the form, and rewrite the templates with the new visual layout.

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Update call to `database.CreateNode` in `CreateNodeHandler` to pass `category` parameter.

#### [MODIFY] [layout.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/layout.templ)
- **Sidebar**:
  - Group nodes in "Mis Nodos" by categories: `MANGA & ANIME`, `IA & RAG`, `GAMING`, and `DESARROLLO & DISEÑO`.
  - Add simple emojis next to node names (⛩️, 🤖, 🧠, 🎮, 🐹, 🎨).
  - Add Tailwind-style class utilities `hover:bg-bg-surface-hover transition-colors rounded-md p-2` to the node items.
- **NavBar**:
  - Make input search style darker with a modern magnifying glass SVG.
  - Add `<kbd class="navbar-search-kbd">Ctrl K</kbd>` indicator.

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- **Collapsible Create Node**:
  - Refactor creation box. Use a fake input box container showing "Crear nueva comunidad..." on collapse.
  - Use simple, lightweight inline JavaScript handlers to toggle the `.expanded` class of the container.
  - In expanded state, render the real form with a Close (X) button, cancel button, and a new `Category` dropdown selector.
- **Welcome Card**:
  - Apply gradient background and brand primary left border styles to `.greeting`.
- **NodeCard**:
  - Refactor class tags to support dynamic category-colored badges.
  - If a node category is empty, render a default "General" badge.

---

### Styling Layer
We will define all required styles, hover animations, shadows, and utility classes in the CSS.

#### [MODIFY] [components.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/components.css)
- **Collapsible Create Box**: Define `.create-node-box`, `.fake-input-box`, `.real-form-box`, `.fake-input-avatar`, `.fake-input-placeholder`, `.form-header`, `.btn-close-compact`, `.form-actions-row` classes with smooth fade-in animations.
- **Welcome Card**: Add left border and linear-gradient background rules to `.greeting`.
- **Node Cards**: Update `.node-card` border, transitions, shadow properties, and category color-coded classes (`.category-manga`, `.category-ia`, `.category-gaming`, etc.).
- **Search input**: Enhance `.navbar-search-input` (darker background, extra padding on right), `.navbar-search-icon` (SVG style support), and `.navbar-search-kbd` (absolutely positioned Ctrl+K badge).
- **Utility Mappings**: Define Tailwind utility styles (`.hover\:bg-bg-surface-hover:hover`, `.transition-colors`, etc.) inside the CSS file to match layout tags.

---

## Verification Plan

### Automated Tests
- Run `go run github.com/a-h/templ/cmd/templ generate` to ensure all `.templ` files compile successfully.
- Run `go test ./...` to verify no Go compilation errors exist.

### Manual Verification
- Launch server locally: `go run ./cmd/nodal/main.go`.
- Check Sidebar categories & icons. Verify hover style feels smooth.
- Click on "Crear nueva comunidad..." input. Confirm that the form expands, and clicking Cancel or close (X) collapses it back.
- Create a new node selecting "IA & RAG" category. Check that the node is created successfully, is persisted in the database with the category, and is rendered on the homepage grid with the correct colored badge.
- Verify NavBar search field appearance (dark background, magnifying glass SVG, Ctrl K badge).
