# Referencia del Design System — Nodal Frontend
# Versión: 1.0 | Mantenido por: Agente Investigador de Frontends
# ─────────────────────────────────────────────────────────────────────────────
# Este documento es la fuente de verdad que el LLM usa como contexto al generar
# componentes UI en Go Templ + HTMX + Tailwind CSS para el proyecto Nodal.
# NUNCA generar componentes sin leer esta referencia primero.
# ─────────────────────────────────────────────────────────────────────────────

## 1. Paleta de Colores (Dark Mode Obligatorio)

El sistema de diseño de Nodal es **exclusivamente oscuro**. Todas las clases de color
deben provenir de esta paleta. Está **PROHIBIDO** usar colores Tailwind por defecto
(`bg-white`, `bg-gray-100`, `text-black`, `bg-blue-500`, etc.) salvo que estén
explícitamente listados aquí como alias.

### Fondos y Superficies
| Token                  | Clase Tailwind           | Valor hex aprox. | Uso                                  |
|------------------------|--------------------------|------------------|--------------------------------------|
| Fondo base             | `bg-nodal-bg`            | `#0d0d0d`        | Fondo de página / layout raíz        |
| Fondo alternativo      | `bg-nodal-bg-alt`        | `#111318`        | Sidebar, panels laterales            |
| Superficie 1           | `bg-nodal-surface`       | `#1a1d27`        | Cards, modales, dropdowns            |
| Superficie 2           | `bg-nodal-surface-2`     | `#22263a`        | Hover state de cards, inputs activos |
| Borde                  | `border-nodal-border`    | `#2d3248`        | Bordes de cards, separadores         |
| Borde sutil            | `border-nodal-border-subtle` | `#1e2235`    | Bordes internos, divisores           |

### Colores Primarios (Acento Índigo-Violeta)
| Token                  | Clase Tailwind              | Valor hex aprox. | Uso                                |
|------------------------|-----------------------------|------------------|------------------------------------|
| Primario base          | `bg-nodal-primary`          | `#6c63ff`        | Botones CTA, links activos         |
| Primario hover         | `bg-nodal-primary-hover`    | `#5a52e0`        | Hover de botón primario            |
| Primario sutil         | `bg-nodal-primary-subtle`   | `#6c63ff1a`      | Badges, highlights de categoría    |
| Texto primario         | `text-nodal-primary`        | `#6c63ff`        | Links, íconos activos              |
| Ring foco              | `ring-nodal-primary`        | `#6c63ff`        | Focus ring de inputs               |

### Textos
| Token              | Clase Tailwind          | Valor hex aprox. | Uso                           |
|--------------------|-------------------------|------------------|-------------------------------|
| Texto base         | `text-nodal-text`       | `#e2e4f0`        | Texto principal               |
| Texto muted        | `text-nodal-text-muted` | `#8b90a7`        | Subtítulos, metadatos, hints  |
| Texto desactivado  | `text-nodal-text-dim`   | `#4a4f6a`        | Placeholders, texto inactivo  |

### Estados Semánticos
| Estado   | Fondo                    | Texto                   | Borde                   |
|----------|--------------------------|-------------------------|-------------------------|
| Success  | `bg-nodal-success-subtle`| `text-nodal-success`    | `border-nodal-success`  |
| Warning  | `bg-nodal-warning-subtle`| `text-nodal-warning`    | `border-nodal-warning`  |
| Error    | `bg-nodal-error-subtle`  | `text-nodal-error`      | `border-nodal-error`    |
| Info     | `bg-nodal-info-subtle`   | `text-nodal-info`       | `border-nodal-info`     |

---

## 2. Tipografía

**Familia base:** `font-inter` (Inter, sans-serif) — definida como `fontFamily.sans` en `tailwind.config.js`.
**Familia mono:** `font-mono` (JetBrains Mono) — para código inline y bloques de código.

### Escala de Tamaños Usada en el Proyecto
| Clase Tailwind | Tamaño rem | Uso típico                              |
|----------------|------------|-----------------------------------------|
| `text-xs`      | 0.75rem    | Metadatos, timestamps, labels de badge  |
| `text-sm`      | 0.875rem   | Texto secundario, descripiciones cortas |
| `text-base`    | 1rem        | Texto principal, párrafos               |
| `text-lg`      | 1.125rem   | Subtítulos de sección, nombres de nodo  |
| `text-xl`      | 1.25rem    | Títulos de card, headings de modal      |
| `text-2xl`     | 1.5rem     | Títulos de página, h1 de sección        |
| `text-3xl`     | 1.875rem   | Hero titles, nombres de Nodo destacado  |

**Pesos:** `font-normal` (400) · `font-medium` (500) · `font-semibold` (600) · `font-bold` (700)

**Line-height por defecto:** `leading-relaxed` para párrafos, `leading-tight` para headings.

---

## 3. Componentes Atómicos Existentes

> **IMPORTANTE:** Antes de generar un nuevo componente, verifica si ya existe uno equivalente
> en esta lista. Si existe, úsalo mediante composición en lugar de re-crearlo.

### 3.1 Botón Primario — `PrimaryButton(label string, hxPost string, hxTarget string)`
```
Clases base: inline-flex items-center gap-2 px-4 py-2 rounded-lg
             bg-nodal-primary hover:bg-nodal-primary-hover
             text-white text-sm font-medium
             transition-colors duration-150
             focus:outline-none focus:ring-2 focus:ring-nodal-primary focus:ring-offset-2 focus:ring-offset-nodal-bg
             disabled:opacity-50 disabled:cursor-not-allowed
HTMX: hx-post hx-target hx-swap="innerHTML" hx-indicator="#spinner"
```

### 3.2 Botón Secundario — `SecondaryButton(label string, href string)`
```
Clases base: inline-flex items-center gap-2 px-4 py-2 rounded-lg
             border border-nodal-border bg-nodal-surface
             text-nodal-text text-sm font-medium
             hover:bg-nodal-surface-2 transition-colors duration-150
             focus:outline-none focus:ring-2 focus:ring-nodal-primary
```

### 3.3 Badge — `Badge(label string, variant string)`
```
Variantes: "primary" | "success" | "warning" | "error" | "neutral"
Clases base: inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
Primario:  bg-nodal-primary-subtle text-nodal-primary border border-nodal-primary/20
Éxito:     bg-nodal-success-subtle text-nodal-success border border-nodal-success/20
Neutral:   bg-nodal-surface-2 text-nodal-text-muted border border-nodal-border
```

### 3.4 Card — `Card(title string, description string)`
```
Clases base: bg-nodal-surface border border-nodal-border rounded-xl p-5
             hover:border-nodal-primary/30 hover:bg-nodal-surface-2
             transition-all duration-200 cursor-pointer
HTMX: hx-get="/nodes/{id}" hx-target="#main-content" hx-push-url="true"
```

### 3.5 Input de Texto — `TextInput(name string, placeholder string, value string)`
```
Clases base: w-full px-3 py-2 rounded-lg
             bg-nodal-surface-2 border border-nodal-border
             text-nodal-text text-sm placeholder:text-nodal-text-dim
             focus:outline-none focus:border-nodal-primary focus:ring-1 focus:ring-nodal-primary
             transition-colors duration-150
```

### 3.6 Textarea — `Textarea(name string, placeholder string, rows int)`
```
Mismas clases que TextInput + resize-none
```

### 3.7 Modal — `Modal(id string, title string)`
```
Overlay:   fixed inset-0 bg-black/60 backdrop-blur-sm z-50
           flex items-center justify-center p-4
Panel:     bg-nodal-surface border border-nodal-border rounded-2xl
           w-full max-w-md p-6 shadow-2xl
Header:    flex items-center justify-between mb-4
           text-nodal-text text-xl font-semibold
HTMX apertura: hx-get="/modal/{id}" hx-target="#modal-container" hx-swap="innerHTML"
HTMX cierre:   hx-on:click="this.closest('[data-modal]').remove()"
```

### 3.8 Toast de Notificación — `Toast(message string, variant string)`
```
Posición:  fixed bottom-4 right-4 z-50
Clases:    flex items-center gap-3 px-4 py-3 rounded-xl
           border shadow-lg text-sm font-medium
           animate-in slide-in-from-bottom-2 duration-300
Success:   bg-nodal-success-subtle border-nodal-success text-nodal-success
Error:     bg-nodal-error-subtle border-nodal-error text-nodal-error
Auto-dismiss: hx-trigger="load delay:3s" hx-swap="outerHTML" hx-get="/empty"
```

### 3.9 Skeleton Loader — `SkeletonCard()` / `SkeletonText(lines int)`
```
Clases:    bg-nodal-surface-2 rounded animate-pulse
Card:      h-32 w-full rounded-xl
Text:      h-4 w-full rounded (repetir N veces con gap-y-2)
Uso:       Como hx-indicator target mientras carga contenido HTMX
```

### 3.10 Avatar — `Avatar(username string, size string)`
```
Tamaños:   "sm" → w-8 h-8 text-xs | "md" → w-10 h-10 text-sm | "lg" → w-14 h-14 text-base
Clases:    rounded-full bg-nodal-primary-subtle border border-nodal-primary/20
           flex items-center justify-center font-semibold text-nodal-primary
           (muestra inicial del username)
```

---

## 4. Convenciones HTMX del Proyecto

### Atributos Permitidos y su Uso
| Atributo          | Uso en Nodal                                             |
|-------------------|----------------------------------------------------------|
| `hx-get`          | Cargar fragmentos HTML (listas, detalles, páginas)       |
| `hx-post`         | Enviar formularios, crear recursos                       |
| `hx-put`          | Actualizar recursos existentes                           |
| `hx-delete`       | Eliminar recursos (con confirmación)                     |
| `hx-trigger`      | Eventos de disparo: `click`, `submit`, `change`, `load`, `revealed` |
| `hx-target`       | Selector CSS del elemento destino (siempre explícito)    |
| `hx-swap`         | **Valores permitidos:** `innerHTML` · `outerHTML` · `beforeend` |
| `hx-indicator`    | Selector del skeleton/spinner mientras carga             |
| `hx-push-url`     | Actualizar URL del navegador en navegación SPA           |
| `hx-boost`        | Activar en `<a>` y `<form>` para navegación sin recarga  |
| `hx-confirm`      | Diálogo de confirmación antes de acciones destructivas   |
| `hx-vals`         | JSON extra a enviar junto al request                     |
| `hx-headers`      | Headers HTTP adicionales (ej. CSRF token)                |
| `hx-on:htmx:after-request` | Callback JS mínimo post-request (solo si es estrictamente necesario) |

### Reglas de Uso HTMX
1. **SIEMPRE especificar `hx-target` explícitamente.** Nunca confiar en el target por defecto.
2. **`hx-swap="innerHTML"`** para actualizar contenido interno de un contenedor.
3. **`hx-swap="outerHTML"`** para reemplazar el elemento completo (ej. Toast, inline-edit).
4. **`hx-swap="beforeend"`** para añadir ítems a listas (infinite scroll, chats).
5. **`hx-indicator`** apunta siempre a un `SkeletonLoader` o spinner con `role="status"`.
6. Los **formularios** siempre usan `hx-post` en el `<form>` element, nunca en el botón.
7. Todos los endpoints HTMX devuelven **fragmentos HTML parciales**, no páginas completas.

---

## 5. Estructura de Archivos .templ

### Plantilla Mínima de Componente
```go
// Componente: NombreComponente
// Descripción: Breve descripción de qué hace este componente.
// Props: - propA (tipo): descripción
//        - propB (tipo): descripción
// HTMX deps: hx-get, hx-target
package components

templ NombreComponente(propA string, propB int) {
	<div class="bg-nodal-surface border border-nodal-border rounded-xl p-4">
		<h2 class="text-nodal-text text-lg font-semibold">{ propA }</h2>
		<p class="text-nodal-text-muted text-sm mt-1">Contenido del componente</p>
	</div>
}
```

### Plantilla Mínima de Página
```go
package pages

import "github.com/tuusuario/nodal/web/components"

templ NombrePagina(data PageData) {
	@components.Layout(data.Title) {
		<main class="flex-1 min-h-screen bg-nodal-bg">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
				<!-- contenido de la página -->
			</div>
		</main>
	}
}
```

### Reglas de Sintaxis Go Templ
- **Package declaration:** siempre primera línea (`package components` o `package pages`).
- **Imports:** solo los estrictamente necesarios; evitar imports no usados.
- **Indentación:** **tabs** (no espacios). El linter de templ lo exige.
- **Expresiones Go:** `{ variable }` para interpolar strings. `@Component(args)` para componer.
- **Atributos dinámicos:** `class={ condicional }` usando `templ.KV()` para clases condicionales.
- **Loops:** `for _, item := range items { @ItemComponent(item) }`.
- **Condicionales:** `if condicion { <div>...</div> } else { <div>...</div> }`.
- **NO usar** `html/template` ni `text/template`. Solo la sintaxis nativa de `templ`.

---

## 6. Reglas de Accesibilidad Mínimas

Todos los componentes generados DEBEN cumplir estas reglas:

| Elemento                  | Regla                                                                    |
|---------------------------|--------------------------------------------------------------------------|
| Botón solo icono          | `aria-label="Descripción de la acción"` obligatorio                      |
| Spinner / skeleton loader | `role="status"` + `aria-label="Cargando..."` + `aria-live="polite"`     |
| Imágenes                  | `alt="descripción"` siempre. Si es decorativa: `alt=""`                  |
| Inputs de formulario      | `<label>` asociado con `for` igual al `id` del input. O `aria-label`.   |
| Modales                   | `role="dialog"` + `aria-modal="true"` + `aria-labelledby` al título     |
| Links de navegación       | Texto descriptivo (no "click aquí"). Si es icono: `aria-label`.         |
| Listas de nodos           | `<ul role="list">` con `<li role="listitem">` para lectores de pantalla |
| Estados de error          | `aria-live="assertive"` en el contenedor de mensajes de error            |

---

## 7. Convenciones de Nomenclatura

| Elemento          | Convención                           | Ejemplo                              |
|-------------------|--------------------------------------|--------------------------------------|
| Componente Templ  | PascalCase                           | `NodeCard`, `UserAvatar`, `ChatInput`|
| Archivo .templ    | snake_case + `.templ`                | `node_card.templ`, `chat_input.templ`|
| Package           | `components` o `pages`              | `package components`                 |
| ID HTML           | kebab-case descriptivo               | `id="node-list"`, `id="chat-feed"`  |
| hx-target         | `#id-descriptivo` (siempre con `#`) | `hx-target="#node-list"`            |
| hx-indicator      | `#skeleton-{contexto}`              | `hx-indicator="#skeleton-nodes"`    |

---

*Última actualización: generado por el Agente Investigador de Frontends — Nodal v1.0*
