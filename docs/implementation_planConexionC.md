# Implementation Plan: User Identity & Discovery (Explore View)

This plan outlines the changes required to display real user identities in the feed and chat (joining with the `users` table) and build the new visual Explore page under `/explore`.

## User Review Required

> [!NOTE]
> Since Nodal uses a static CSS system, we will define custom CSS utility mappings inside `components.css` to support the required Tailwind-style responsive grids (`grid-cols-2 md:grid-cols-3 lg:grid-cols-4`), gradients (`bg-gradient-to-t from-black/90 via-black/40 to-transparent`), aspect ratio (`aspect-square`), and scale hover effects.

---

## Proposed Changes

### Database Layer

#### [MODIFY] [node_repo.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/platform/database/node_repo.go)
- Add `Username string `json:"username"`` and `UserInitial string `json:"user_initial"`` to the `Node` and `ChatMessage` structs.
- Update `CreateNode` signature to accept `ownerID *string` and persist it in the SQL `INSERT` statement.
- Update `ListNodes` and `GetNodeByID` to perform a `LEFT JOIN users u ON n.owner_id = u.id`, select the creator's username, and populate `Username` and `UserInitial` on Go structures.
- Update `CreateChatMessage` signature to accept `userID *string` and insert it.
- Update `ListChatMessages` to `LEFT JOIN users u ON c.user_id = u.id` and select the author's username.

---

### Backend Controller & Routing

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Extract the authenticated `UserID` from context (`middleware.ClaimsFromContext(r.Context())`) inside `CreateNodeHandler` and pass it to `database.CreateNode`.
- In `PostChatMessageHandler`, extract the authenticated `UserID` (either from the request context or by fallback cookie parsing) and pass it to `database.CreateChatMessage`.

#### [MODIFY] [home.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/home.go)
- Add `ExploreHandler(db)` to query all nodes via `database.ListNodes` and render the `views.Explore` template page.

#### [MODIFY] [main.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/cmd/nodal/main.go)
- Map the route `GET /explore` to `handlers.ExploreHandler(db)`.

---

### Frontend Views & CSS Mappings

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- Update `@NodeCard` to render the creator's username (`u/{ n.Username }`) and the first initial (`n.UserInitial`).

#### [MODIFY] [node_detail.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/node_detail.templ)
- Update `@ChatMessageItem` to display the actual author's username and their initial instead of the static `"Anónimo"` / `"A"`.

#### [MODIFY] [layout.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/layout.templ)
- Update the sidebar "Explorar" link `href` attribute from `/nodes` to `/explore`.

#### [NEW] [explore.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/explore.templ)
- Implement `Explore` template wrapping all nodes in a grid: `<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">`.
- Implement `ExploreCubeItem` template as a square cover card (`aspect-square`) with a dark bottom gradient overlay (`bg-gradient-to-t from-black/90 via-black/40 to-transparent`) and superimposed white text.

#### [MODIFY] [components.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/components.css)
- Add Tailwind CSS utility mapping declarations for grid properties, aspect ratio, hover transitions, scale transforms, and black gradient configurations to support the Explore grid elements.

---

## Verification Plan

### Automated Verification
- Run `templ generate` to compile the explore page and modifications.
- Run `go build ./...` to verify there are no compilation errors.

### Manual Verification
- Log in to the platform, create a new Node, and send chat messages.
- Confirm that the username and avatar initial are correctly displayed.
- Navigate to `/explore` (via the sidebar "Explorar" link) and verify the responsive cover grid of Node cubes is displayed.
