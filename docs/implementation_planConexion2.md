# Implementation Plan: Backend Integration Phase 2 (Multimedia Support)

This plan details the steps required to implement node cover image uploads, database schema/query integration, and feed card presentation changes.

## User Review Required

> [!IMPORTANT]
> The database migration adding `image_url` to the `nodes` table has already been successfully executed on the active Docker PostgreSQL container. We will also persist this change in `migrations/init.sql` for future schema setup.
> 
> A compilation of the frontend templ templates (`templ generate`) is required after modifying the `.templ` source files.

## Proposed Changes

### Database Layer

#### [MODIFY] [node_repo.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/platform/database/node_repo.go)
- Add `ImageURL *string `json:"image_url"`` to the `Node` struct.
- Update `CreateNode` signature to `CreateNode(db *sql.DB, title, description, category string, imageURL *string) (string, error)` and include `image_url` in the SQL `INSERT` statement.
- Update the SQL query in `ListNodes` to include `image_url` in the fields scanned.
- Update the SQL query in `GetNodeByID` to include `image_url` in the fields scanned.

#### [MODIFY] [init.sql](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/migrations/init.sql)
- Add `image_url TEXT` column to the `nodes` table definition to keep the database initialization script up to date.

---

### Backend Controller

#### [MODIFY] [node.go](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/node.go)
- Add imports: `"os"`, `"io"`, `"path/filepath"`.
- Modify `CreateNodeHandler` to process multipart form data with a 10MB limit: `r.ParseMultipartForm(10 << 20)`.
- Extract file from `image` form field:
  - If a file is uploaded, create the directory `./static/uploads/` (if it does not exist) and write the file using a safe, collision-free filename (using `time.Now().UnixNano()`).
  - Pass the relative file path (e.g., `/static/uploads/img-12345.png`) to `database.CreateNode`.
  - If no file is uploaded, pass `nil` to `database.CreateNode`.

---

### Frontend Views & Templates

#### [MODIFY] [home.templ](file:///c:/Users/Estefi/Desktop/ProyectosGitHub/NebriSocial/Nebrisocial/internal/handlers/views/home.templ)
- Update the Node creation `<form>` tag to include the `enctype="multipart/form-data"` attribute.
- Add the `<input type="file" name="image" accept="image/*" class="w-full bg-bg-elevated border border-border-subtle rounded-md p-2 text-text-primary text-sm" />` element directly below the description textarea inside the form.
- Update `@NodeCard` to render the uploaded image using an `<img ...>` tag conditionally if `n.ImageURL` is not nil or empty.
- Remove the legacy generic grey "Espacio para Imagen/Video" placeholder container.

---

## Verification Plan

### Automated Verification
- Run `templ generate` to compile the `.templ` changes into Go code.
- Run `go build ./...` to verify there are no compilation errors or signature mismatches.

### Manual Verification
- Deploy/start the server.
- Create a new node with an image uploaded, verify it uploads to `./static/uploads/`, and confirm it displays correctly on the home feed without the grey placeholder.
