# Implementation Plan - Monochromatic Minimalist Pivot & Light/Dark Theme Support

This plan details the steps required to pivot the Nodal UI into a Monochromatic Minimalist design (White, Black, and Greyscale) with support for Light and Dark modes.

## User Review Required

> [!IMPORTANT]
> - All purple brand colors will be removed and replaced with a monochromatic palette.
> - High-contrast buttons (white-on-black in dark mode, black-on-white in light mode) will replace the violet gradient CTAs.
> - A theme switch toggle button (sun/moon icon) will be added to the NavBar, backed by an inline JS script to prevent theme flashing on load.
> - The large greeting/hero landing block will be removed for logged-in and anonymous users, allowing the nodes feed to start directly.

## Proposed Changes

### 1. Theme & Styles

#### [MODIFY] [design_system.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub\NebriSocial/Nebrisocial/static/css/design_system.css)
- Remove all purple/brand colors.
- Define theme variables in `:root` (Light Mode):
  - `--bg-base`: `#ffffff`
  - `--bg-surface`: `#f4f4f5`
  - `--bg-elevated`: `#fafafa`
  - `--border-subtle`: `#e4e4e7`
  - `--border-default`: `#e4e4e7`
  - `--text-primary`: `#09090b`
  - `--text-secondary`: `#71717a`
  - `--btn-primary-bg`: `#09090b`
  - `--btn-primary-text`: `#ffffff`
  - `--btn-secondary-bg`: `#ffffff`
  - `--btn-secondary-text`: `#09090b`
- Define theme variables in `html[data-theme="dark"]` (Dark Mode):
  - `--bg-base`: `#000000`
  - `--bg-surface`: `#121212`
  - `--bg-elevated`: `#18181b`
  - `--border-subtle`: `#27272a`
  - `--border-default`: `#27272a`
  - `--text-primary`: `#fafafa`
  - `--text-secondary`: `#a1a1aa`
  - `--btn-primary-bg`: `#fafafa`
  - `--btn-primary-text`: `#000000`
  - `--btn-secondary-bg`: `#121212`
  - `--btn-secondary-text`: `#fafafa`
- Update `--brand-` and `--gradient-` aliases to map to scale-of-grey values matching the active theme, preserving compatibility with existing CSS classes.
- Update global scrollbar styles for `html` and `body` to target `scrollbar-color: var(--border-subtle) var(--bg-base)` and map webkit scrollbars directly to `html`.

#### [MODIFY] [components.css](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/static/css/components.css)
- Update primary buttons `.btn-primary` to use `--btn-primary-bg` and `--btn-primary-text`.
- Update secondary buttons `.btn-secondary` to use `--btn-secondary-bg`, `--btn-secondary-text` and `--border-subtle`.
- Add `.btn-theme-toggle` class for the NavBar theme toggle button.
- Remove purple accents from card headers and dividers.
- Update NodeCard hover state to transition background seamlessly.
- Simplify/remove the heavy `.hero` CSS since the feed starts directly.

### 2. Layout & Page Templates

#### [MODIFY] [layout.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/layout.templ)
- Insert a script block in `<head>` to read `localStorage.getItem('theme')` and immediately call `document.documentElement.setAttribute('data-theme', theme)` to avoid render flashing.
- Add theme toggler SVG icons (`theme-toggle-dark-icon` and `theme-toggle-light-icon`) and `toggleTheme()` trigger button in the NavBar actions.
- Implement the inline script to update icon visibility on `DOMContentLoaded` and HTMX page swaps (`htmx:afterSwap`).

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- Completely delete the large `.hero` section for non-authenticated users.
- Replace it with a minimal inline warning/header card if not authenticated, letting the nodes list start directly on load.
- Ensure the vertical feed items `NodeCard` fill the feed column width transparently with only bottom borders (`border-b border-border-subtle`) and no card border-radius.

## Verification Plan

### Automated Tests & Compiles
- Run `templ generate`
- Run `go build ./...`
- Restart the server process.

### Manual Verification
- Open Nodal in browser.
- Verify Light Mode displays high contrast theme (white background, dark grey accents).
- Toggle Theme button and verify it switches instantly to Dark Mode (pure black background, light grey accents).
- Verify state is saved in LocalStorage and persists across page reloads.
- Verify the NodeCards feed is seamless (no full borders or rounded shapes, only subtle bottom borders).
- Verify that the Hero block is removed.
