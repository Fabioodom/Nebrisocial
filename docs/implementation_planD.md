# Implementation Plan - Premium Dark Mode Refactoring

This plan details the steps required to refactor the Nodal frontend into a Premium Social Network with an elegant Dark Mode (inspired by Vercel, Linear, and Reddit) and a three-column layout.

## User Review Required

> [!IMPORTANT]
> The refactor changes the visual layout to a 3-column architecture, limits the feed width to 680px, and changes the global color palette to almost-black and dark anthracite. This will completely replace the previous layout and style.

## Proposed Changes

### 1. Style & Design Tokens

#### [MODIFY] [design_system.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/design_system.css)
- Update CSS variables in `:root` to apply the Premium Dark Mode colors:
  - `--bg-base`: `#0a0a0a` (almost pure black)
  - `--bg-surface`: `#121212` (dark anthracite)
  - `--bg-surface-hover`: `#1e1e1e` (soft hover grey)
  - `--bg-elevated`: `#1a1a1a` (slightly lighter surface element)
  - `--border-subtle`: `rgba(255, 255, 255, 0.08)` (semi-invisible border)
  - `--border-default`: `rgba(255, 255, 255, 0.08)`
  - `--text-primary`: `#ededed` (off-white text)
  - `--text-secondary`: `#a1a1aa` (muted text)
- Implement custom scrollbars globally using `::-webkit-scrollbar` (transparent background, invisible track, rounded dark grey thumb `#333`).

#### [MODIFY] [components.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/components.css)
- Implement custom utility and layout classes to support the 3-column architecture under `.app-body`.
- Refactor the `.sidebar` to have `transparent` background and `250px` width.
- Refactor the feed container `.feed-container` to center it with `max-width: 680px` and set padding.
- Add `.widgets-sidebar` styles: `width: 300px`, hidden on mobile and visible on desktop (at screen width `1024px` / `lg` breakpoint).
- Refactor `.node-card` to style cards as a vertical stack: transparent background, generous padding (`p-4`), border-bottom, and transitions.
- Add `.ai-embed` styles: background `var(--bg-surface)`, left border of `4px` with `--ai-primary`, small border-radius, and proper padding.
- Style the chat input area to be sticky at the bottom (`position: sticky; bottom: 0`), pill-shaped (`border-radius: var(--radius-pill)`), and styled minimalist send button.
- Clean up other component styling (such as buttons and headers) to fit the new colors.

### 2. Layout & Page Templates

#### [MODIFY] [layout.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/layout.templ)
- Update `Layout()` and `LayoutWithMarkdown()` to configure a flex layout container underneath the `NavBar` with a maximum width of `1600px` centered via `mx-auto`.
- Add a new Right Column (`.widgets-sidebar`) inside `layout.templ` containing a mockup panel of "Nodos Populares" or "Trending".

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- Remove the server status card ("Estado del Servidor") and checking logic.
- Replace the expanded creation box with a compact, one-liner creation bar: simulated circular avatar at the left, fake input "Crear nueva comunidad o hilo...", and hide the real form until clicked (using a `<details>` element or simple toggling logic).
- Refactor `NodeGrid` and `NodeCard` into a vertical stack (`flex flex-col gap-0`) instead of a grid, applying the transparent card design.

#### [MODIFY] [node_detail.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/node_detail.templ)
- Wrap AI-generated summary cards inside the new `.ai-embed` container.
- Update the chat form input textarea and submit button to adopt the sticky pill-shaped design, and use a minimalist icon-only send button.
- Format Markdown headings and paragraphs with generous margins and line-heights.

### 3. Backend Enhancements

#### [MODIFY] [home.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/home.go)
- Query the database with `database.FindUserByID` if the user is authenticated to obtain their actual username, avoiding showing the raw UUID in the greeting.

## Verification Plan

### Automated Tests & Compiles
- Generate templ components: `templ generate`
- Run the compiler: `go build ./cmd/nodal`
- Run the server locally and check for any startup errors.

### Manual Verification
- Open `http://localhost:8080/` in browser.
- Verify the 3-column layout structure.
- Verify the Dark Mode colors (bg-base: #0a0a0a, bg-surface: #121212, text-primary: #ededed).
- Verify the compact creation bar expands on click.
- Verify the vertical feed items and premium card hover effects.
- Verify the AI summary styling and markdown margins on node details.
