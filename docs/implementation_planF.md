# Implementation Plan - Premium Layout & UX Refactor

Elevate the user experience and visual design of Nodal by making it fully responsive (mobile-first), using professional SVG iconography instead of emojis in the navigation, moving authentication controls to the global NavBar, and organizing the node detail view into a clean two-column layout separating user chat and AI summary updates.

## User Review Required

> [!IMPORTANT]
> The template signature of `Layout`, `LayoutWithMarkdown`, and `NavBar` will be updated to accept `isAuthenticated bool, username string`. This requires updating all callers (`home.templ`, `auth.templ`, `audit.templ`, `node_detail.templ`) to pass these parameters.
> The `/profile` handler will be registered at `/profile` and will render a stub profile with tabs "Mis Nodos", "Guardados", and "Publicaciones".

## Proposed Changes

### Component: Core Layout & NavBar Refactor

#### [MODIFY] [layout.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/layout.templ)
- Update `Layout` and `LayoutWithMarkdown` parameters: `Layout(isAuthenticated bool, username string)`.
- Update `NavBar` parameter list to take authentication state.
- In `NavBar`, add the hamburger button `<button class="navbar-hamburger md:hidden">` on the left.
- In `NavBar`, remove the `Auditoría IA` button and instead build a dropdown/actions group on the right:
  - If NOT authenticated: Show "Iniciar sesión" and "Registrarse" buttons.
  - If authenticated: Show a link to `/profile` with a rounded user profile avatar (first letter of the username or user icon) and the username, along with a "Cerrar sesión" button.
- In `Sidebar`, replace all emojis with clean, professional SVG icons inline (e.g. Home, Compass, Sparkles, Brain, CPU, Gamepad, Terminal, Palette).
- Implement `toggleSidebar()` JavaScript globally in the template.

#### [NEW] [profile.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/profile.templ)
- Create a stub Profile view inside the layout, featuring user headers, tabs ("Mis Nodos", "Guardados", "Publicaciones") and empty states.

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- Update `@Layout()` call to `@Layout(isAuthenticated, username)`.
- Remove the Iniciar Sesión/Registrarse and logout buttons from the feed-header.
- Redesign `NodeCard` to be an enriched post card with a header (user, timestamp, category), clickable title, description text, a 256px tall multimedia placeholder, and fake action buttons (Like, Comment, Share) using SVGs.

#### [MODIFY] [auth.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/auth.templ)
- Update `@Layout()` calls to `@Layout(false, "")` since the user is not authenticated on login/register views.

#### [MODIFY] [audit.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/audit.templ)
- Update template definition to accept `isAuthenticated bool, username string`.
- Pass these variables into the `@Layout` call: `@Layout(isAuthenticated, username)`.

### Component: Backend Handlers

#### [MODIFY] [home.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/home.go)
- Define a `ProfileHandler` that checks authentication (using the session cookie), fetches user details, and renders the profile template. Redirects unauthorized requests to `/login`.

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- In `NodeDetailHandler`, fetch the user's username if they are authenticated, and pass both `isAuthenticated` and `username` to `views.NodeDetail`.

#### [MODIFY] [audit.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/audit.go)
- Check authentication state/username in `AuditHandler` and pass them to the `AuditDashboard` view.

#### [MODIFY] [main.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/cmd/nodal/main.go)
- Register route `mux.HandleFunc("/profile", handlers.ProfileHandler(db))` so users can access their profile page.

### Component: Chat Layout & Agente Cronista

#### [MODIFY] [node_detail.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/node_detail.templ)
- Update the component signature to include `username string`.
- Update the layout call to `@LayoutWithMarkdown(isAuthenticated, username)`.
- Split the container into two columns on desktop:
  - Left Column (70%): Chat messages timeline `#chat-messages` (looping only over messages, excluding threads) and the message input area at the bottom.
  - Right Column (30%): A sticky sidebar container displaying only AI-generated summary threads from the Agente Cronista, with a header, nice cards, and a description.
- Implement helper component `@CronistaThreadItem(t *database.Thread)`.

### Component: Stylesheets (CSS)

#### [MODIFY] [components.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/components.css)
- Implement layout responsiveness using mobile-first styles:
  - Add standard responsive utility classes (e.g. `.hidden`, `@media (min-width: 768px) { .md\:flex { display: flex !important; } .md\:hidden { display: none !important; } }` etc.).
  - Style mobile hamburger button and mobile drawer sidebar behavior (slide in from left using `@keyframes slideInLeft`).
  - Set default styles for mobile views: hide left sidebar and right widgets sidebar (`hidden md:flex`, `hidden lg:flex`).
  - Feed container full width on mobile (`w-full px-4`).
- Add premium styles for the enriched `NodeCard`: hover elevations, subtle borders, shadows, and spacing.
- Add CSS grid styles for Node Detail: 2-column flexbox layout on desktop, sticky column behavior, and professional cards for AI summaries.
- Style `/profile` page elements: avatar initials, tabs with indicator underlines, and empty states.

## Verification Plan

### Automated Tests
- Build templates: `templ generate`
- Compile and run server: `go run cmd/nodal/main.go`
- Ensure no compilation or parsing errors occur.

### Manual Verification
- Test responsiveness in the browser (by resizing the window or toggling mobile device mode in Developer Tools):
  - Check that sidebars disappear on mobile and the feed becomes full-width.
  - Check that the hamburger button is visible on mobile and toggles the sidebar drawer successfully.
- Log in and verify that the navbar updates to show `@username` and the profile avatar initials.
- Click the profile avatar and ensure `/profile` loads correctly, displaying the skeleton with tabs.
- Open a node detail page, check that it divides into two columns on desktop, and check that the AI summaries reside entirely in the right-side sticky panel.
- Ensure that Nodal still compiles and loads fast.
