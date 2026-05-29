# Frontend Master Plan — Nodal
> **Rol:** Lead Frontend Architect | **Versión:** 1.0.0 | **Fecha:** 27 mayo 2026 | **Estado:** Borrador Arquitectónico

---

## 1. Análisis de Patrones — Plataformas de Referencia

### 1.1 Extracción de Patrones Clave

| Patrón | Reddit | Discord | StackOverflow | Traducción a Nodal |
|---|---|---|---|---|
| **Navegación de Comunidades** | Barra lateral fija con lista de subreddits suscritos | Panel izquierdo con servidores/canales siempre visibles | Categorías en tabs horizontales | **Sidebar izquierda**: lista de Nodos unidos + Explorar |
| **Jerarquía de contenido** | Feed → Thread → Respuestas | Servidor → Canal → Mensajes | Pregunta → Respuestas → Comentarios | **Nodo → Timeline (chat + IA) → Thread Replies** |
| **Modo dual de comunicación** | Solo foro asíncrono | Solo chat síncrono | Solo foro Q&A | **Nodal fusiona ambos**: chat en vivo + hilos persistentes |
| **Señales de actividad** | Conteo de members online | Dot verde de presencia | Contador de respuestas | Contadores en sidebar: miembros + mensajes/día |
| **Onboarding** | r/popular como landing | Explorar servidores públicos | Sin onboarding de comunidad | Hero → Grid de Nodos → CTA de unirse |
| **Identidad de comunidad** | Banner + avatar de subreddit | Icono de servidor | Sin identidad visual propia | Header de Nodo con gradient + metadata badge |
| **Entrada de mensaje** | Bottom-fixed textarea | Bottom-fixed input | No aplica | `chat-input-area` fijo al fondo del panel |
| **Contenido generado por IA** | No existe | No existe | No existe | `ai-summary-card` con badge visual diferenciado |

### 1.2 Decisión Arquitectónica Central

> **Nodal NO es Reddit ni Discord. Es ambos simultáneamente.**
>
> La UI debe transmitir que el Nodo es un espacio vivo (chat) con memoria permanente (hilos IA). El usuario debe sentir que **entrar a un Nodo es como entrar a una sala que recuerda todo lo que se ha dicho**.

### 1.3 UX de Primera Impresión (First-Touch UX)

**Estado: No autenticado**
```
Landing → Hero con tagline → Grid de Nodos públicos visibles → CTA doble (Registrarse / Login)
```
*Patrón tomado de Reddit: mostrar contenido antes de pedir registro, reduciendo la fricción.*

**Estado: Autenticado, sin Nodos**
```
Dashboard → Saludo personalizado → "Explorar Nodos" destacado → Form de creación accesible
```
*Patrón tomado de Discord: onboarding activo, no muro vacío.*

**Estado: Autenticado, dentro de Nodo**
```
Sidebar (lista Nodos) + Node Header (identidad) + Timeline (chat+IA) + Input fijo
```
*Patrón tomado de Discord (estructura) + Reddit (contenido persistente).*

---

## 2. Estructura de Componentes

### 2.1 Árbol de Componentes Templ

```
views/
├── layout.templ                    # [EXTRAER] Layout base actualmente en home.templ
│   ├── Layout()                    # Shell HTML: head, htmx CDN, estilos globales, nav
│   └── NavBar()                    # Navbar: brand + auth actions + audit link
│
├── home.templ                      # Página principal
│   ├── Home(isAuth, user, nodes)   # Orquestador: decide qué renderizar
│   ├── HeroSection()               # Landing para no-auth: tagline + CTA
│   ├── Dashboard(user)             # Saludo autenticado + acciones rápidas
│   ├── NodeCreateForm()            # Form HTMX para crear nodo
│   └── NodeGrid(nodes)             # Grid de node-cards
│       └── NodeCard(node)          # Card individual de nodo
│
├── node_detail.templ               # Vista de nodo (ya existe, refactorizar)
│   ├── NodeDetail(node, msgs, ...)  # Orquestador de la vista de nodo
│   ├── NodeHeader(node)            # Header con título, categoría, meta
│   ├── NodeTimeline(items)         # Contenedor scrollable del timeline
│   │   ├── ChatMessageItem(msg)    # Burbuja de mensaje de chat
│   │   └── AISummaryCard(thread)  # Tarjeta de resumen IA con Markdown
│   └── ChatInputArea(nodeID, auth) # Form de envío + cronista hint
│
├── node_list.templ                 # [NUEVO] Página de exploración de Nodos
│   ├── NodeExplorer()              # Layout de exploración
│   ├── NodeSearchBar()             # Input de búsqueda con hx-get
│   └── NodeResults(nodes)          # Resultados (target de búsqueda HTMX)
│
├── thread_view.templ               # [NUEVO] Vista de hilo de foro
│   ├── ThreadView(thread, replies) # Hilo completo
│   ├── ThreadHeader(thread)        # Título, autor, fecha, badge IA
│   ├── ThreadBody(thread)          # Cuerpo en Markdown
│   ├── ReplyList(replies)          # Lista de respuestas
│   │   └── ReplyItem(reply)        # Respuesta individual
│   └── ReplyForm(threadID)         # Form para responder
│
├── auth.templ                      # Autenticación (ya existe)
│   ├── LoginPage()
│   └── RegisterPage()
│
└── audit.templ                     # Dashboard de auditoría IA (ya existe)
    └── AuditDashboard(logs)
```

### 2.2 Componentes Fragmento (Respuestas HTMX Parciales)

Estos componentes son fragmentos HTML puros devueltos por endpoints específicos:

```
fragments/
├── ChatMessageFragment(msg)        # POST /nodes/:id/chat → beforeend #chat-messages
├── NodeCardFragment(node)          # POST /nodes → afterbegin #nodes-grid
├── SearchResultsFragment(nodes)    # GET /nodes/search → innerHTML #node-results
├── HealthFragment(status)          # GET /health → innerHTML #health-response
└── GuardianResponseFragment(res)   # POST /nodes (validación) → innerHTML #guardian-feedback
```

---

## 3. Flujo de Navegación y Comportamiento HTMX

### 3.1 Mapa de Rutas y Estrategia HTMX

```
GET  /                → Full page render (Layout + Home)
GET  /nodes           → Full page render (Layout + NodeExplorer)
GET  /nodes/search    → Fragmento: SearchResultsFragment  [hx-target="#node-results"]
GET  /nodes/:id       → Full page render (Layout + NodeDetail)
POST /nodes           → Fragmento: GuardianResponseFragment → si OK: redirect
POST /nodes/:id/chat  → Fragmento: ChatMessageFragment    [hx-target="#chat-messages", swap=beforeend]
GET  /nodes/:id/threads       → Full page render (tab en NodeDetail)
GET  /nodes/:id/threads/:tid  → Full page render (ThreadView)
POST /nodes/:id/threads/:tid/reply → Fragmento: ReplyItem [hx-target="#replies-list", swap=beforeend]
GET  /register        → Full page render (Layout + RegisterPage)
GET  /login           → Full page render (Layout + LoginPage)
POST /auth/logout     → hx-swap="none" → JS redirect
GET  /admin/audit     → Full page render (Layout + AuditDashboard)
GET  /health          → Fragmento de texto                [hx-target="#health-response"]
```

### 3.2 Estrategias de Swap por Contexto

| Acción | `hx-swap` | `hx-target` | Razón |
|---|---|---|---|
| Enviar mensaje de chat | `beforeend` | `#chat-messages` | Agregar al final del timeline |
| Buscar Nodos | `innerHTML` | `#node-results` | Reemplazar resultados completos |
| Crear Nodo exitoso | `innerHTML` | `#guardian-feedback` | Mostrar validación, luego redirigir |
| Crear Nodo en grid | `afterbegin` | `#nodes-grid` | Insertar al inicio para visibilidad |
| Responder en hilo | `beforeend` | `#replies-list` | Agregar respuesta al final |
| Health check | `innerHTML` | `#health-response` | Mostrar estado actual |
| Logout | `none` | — | Solo acción, JS maneja redirect |

### 3.3 Flujo de Navegación con hx-boost

```html
<!-- En Layout, activar hx-boost para toda la app -->
<body hx-boost="true">
```

**Comportamiento con `hx-boost="true"`:**
- Todos los `<a href>` dentro del body hacen AJAX en lugar de full reload
- HTMX intercambia solo el `<body>` de la respuesta del servidor
- El historial del navegador se actualiza (`history.pushState`)
- La navbar, scripts y estilos globales NO se recargan

**Excepciones que requieren `hx-boost="false"`:**
- Links a OAuth externo (Google, GitHub)
- Links de descarga de archivos
- Logout (usa `hx-post` directo, no navegación)

### 3.4 Flujo Específico: Creación de Nodo con Guardián IA

```
1. Usuario escribe título + descripción
2. POST /nodes con hx-target="#guardian-feedback"
   │
   ├─ [Similitud > 0.95] → GuardianResponseFragment: BLOQUEADO
   │   Muestra: "Ya existe un Nodo similar: [link al nodo]"
   │   Color: rojo. No hay redirect.
   │
   ├─ [Similitud 0.85-0.95] → GuardianResponseFragment: SUGERENCIA
   │   Muestra: "Nodos similares encontrados: [lista]. ¿Crear de todas formas?"
   │   Color: amarillo. Botón de confirmación con hx-post adicional.
   │
   └─ [Similitud < 0.85 o aprobado] → redirect 303 a /nodes/:id
       HTMX maneja el redirect automáticamente (hx-boost activo)
```

### 3.5 Flujo Específico: Chat en Tiempo Real

```
Fase MVP (polling):
GET /nodes/:id/chat/poll?after={last_msg_id}
    hx-trigger="every 3s"
    hx-target="#chat-messages"
    hx-swap="beforeend"

Fase Post-MVP (WebSocket nativo):
ws://host/nodes/:id/ws
    → Go handler escribe ChatMessageFragment HTML por el WS
    → HTMX WS Extension: hx-ws="connect:/nodes/:id/ws"
    → hx-swap-oob="beforeend:#chat-messages"
```

---

## 4. Sistema de Diseño Consolidado

### 4.1 Paleta de Colores (Tokens Semánticos)

El design system actual usa CSS-in-Templ inline. El objetivo es extraerlo a `static/css/design_system.css`.

```css
/* ── TOKENS PRIMITIVOS ─────────────────────────────────── */
:root {
  /* Backgrounds */
  --bg-base:          #0f0f13;
  --bg-surface:       rgba(255,255,255,0.04);
  --bg-surface-hover: rgba(255,255,255,0.07);
  --bg-elevated:      rgba(255,255,255,0.06);
  --bg-overlay:       rgba(0,0,0,0.15);

  /* Borders */
  --border-subtle:  rgba(255,255,255,0.08);
  --border-default: rgba(255,255,255,0.12);
  --border-active:  rgba(124,58,237,0.35);

  /* Texto */
  --text-primary:   #e2e8f0;
  --text-secondary: #94a3b8;
  --text-muted:     #64748b;
  --text-disabled:  #475569;

  /* Brand */
  --brand-primary:  #7c3aed;
  --brand-accent:   #a855f7;
  --brand-soft:     #a78bfa;
  --brand-subtle:   #c4b5fd;

  /* AI / Cronista */
  --ai-primary:  #6366f1;
  --ai-soft:     #818cf8;
  --ai-subtle:   #a5b4fc;
  --ai-bg:       rgba(99,102,241,0.10);
  --ai-border:   rgba(99,102,241,0.30);

  /* Estados */
  --success:     #6ee7b7;
  --danger:      #f87171;
  --danger-bg:   rgba(239,68,68,0.12);
  --warning:     #fbbf24;
  --warning-bg:  rgba(251,191,36,0.10);

  /* Gradientes */
  --gradient-brand:       linear-gradient(135deg, #7c3aed, #a855f7);
  --gradient-title:       linear-gradient(135deg, #a78bfa, #818cf8);
  --gradient-node-header: linear-gradient(135deg, rgba(124,58,237,0.15), rgba(168,85,247,0.08));
}

/* ── TOKENS TIPOGRÁFICOS ───────────────────────────────── */
:root {
  --font-sans:      -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-size-xs:   0.72rem;
  --font-size-sm:   0.82rem;
  --font-size-base: 0.9rem;
  --font-size-md:   1rem;
  --font-size-lg:   1.1rem;
  --font-size-xl:   1.25rem;
  --font-size-2xl:  1.75rem;
  --font-size-3xl:  2.5rem;
}

/* ── TOKENS DE ESPACIADO ───────────────────────────────── */
:root {
  --space-1: 0.25rem;  --space-2: 0.5rem;
  --space-3: 0.75rem;  --space-4: 1rem;
  --space-6: 1.5rem;   --space-8: 2rem;
  --space-12: 3rem;
}

/* ── TOKENS DE RADIO ───────────────────────────────────── */
:root {
  --radius-sm: 6px;   --radius-md: 8px;
  --radius-lg: 12px;  --radius-xl: 14px;
  --radius-2xl: 16px; --radius-pill: 99px;
}
```

### 4.2 Catálogo de Componentes CSS

#### Botones

| Clase | Uso | Estado actual |
|---|---|---|
| `.btn-primary` | Acción principal | ✅ Existe |
| `.btn-secondary` | Acciones secundarias | ✅ Existe |
| `.btn-danger` | Logout, eliminar | ✅ Existe |
| `.btn-ghost` | Acciones terciarias en sidebar | ❌ Crear |
| `.btn-oauth` | Login OAuth | ✅ Existe |
| `.btn-audit-nav` | Acceso a panel IA | ✅ Existe |

#### Cards

| Clase | Uso | Estado actual |
|---|---|---|
| `.card` | Contenedor genérico | ✅ Existe |
| `.node-card` | Card de nodo en grid | ✅ Existe |
| `.ai-summary-card` | Resumen IA en timeline | ✅ Existe |
| `.thread-card` | Hilo en lista de threads | ❌ Crear |
| `.resource-card` | Link/recurso compartido | ❌ Crear |

#### Navegación

| Clase | Uso | Estado actual |
|---|---|---|
| `.navbar` | Barra superior | ✅ Existe |
| `.sidebar` | Panel izquierdo de Nodos | ❌ Crear |
| `.sidebar-node-item` | Item de nodo en sidebar | ❌ Crear |
| `.breadcrumb` | Ruta de navegación | ✅ Existe |
| `.tab-bar` | Tabs en vista de Nodo | ❌ Crear |

#### Chat

| Clase | Uso | Estado actual |
|---|---|---|
| `.chat-section` | Contenedor del chat | ✅ Existe |
| `.chat-timeline` | Área scrollable | ✅ Existe |
| `.chat-message` | Mensaje individual | ✅ Existe |
| `.chat-message-bubble` | Burbuja de texto | ✅ Existe |
| `.chat-input-area` | Área de input fija | ✅ Existe |

### 4.3 Layout System

```
Estructura General (post-refactor):

┌──────────────────────────────────────────────────────────┐
│  NAVBAR  (100vw, fixed top)                              │
├──────────┬───────────────────────────────────────────────┤
│          │                                               │
│ SIDEBAR  │  MAIN CONTENT AREA                            │
│ (260px,  │  (flex: 1, max-width: 860px en páginas feed) │
│  fixed)  │                                               │
│          │                                               │
└──────────┴───────────────────────────────────────────────┘

Vista de Nodo (NodeDetail):
┌──────────┬────────────────────────────────┐
│          │  NODE HEADER                   │
│ SIDEBAR  ├────────────────────────────────┤
│          │  TAB BAR: [Chat] [Hilos] [Rec] │
│          ├────────────────────────────────┤
│          │  TIMELINE (scrollable)         │
│          │  - ChatMessageItem × N         │
│          │  - AISummaryCard × N           │
│          ├────────────────────────────────┤
│          │  CHAT INPUT AREA (fixed)       │
└──────────┴────────────────────────────────┘
```

### 4.4 Animaciones y Micro-interacciones

| Animación | Trigger | Duración | Componente |
|---|---|---|---|
| `fadeIn` | Inserción de nuevo mensaje | 250ms | `ChatMessageItem` |
| `fadeIn` con translateY | Inserción de AISummary | 400ms | `AISummaryCard` |
| `translateY(-2px)` hover | Hover sobre NodeCard | 150ms | `NodeCard` |
| `translateY(-1px)` hover | Hover sobre btns primarios | 200ms | `.btn-primary` |
| Skeleton shimmer | Loading estados HTMX | Continua | `hx-indicator` global |
| Border pulse | Foco en inputs | 200ms | `.form-group input:focus` |

### 4.5 Indicadores de Carga HTMX

```css
.htmx-indicator { display: none; }
.htmx-request .htmx-indicator { display: flex; }
.htmx-request.htmx-indicator { display: flex; }

.loader-spinner {
  width: 18px; height: 18px;
  border: 2px solid var(--border-subtle);
  border-top-color: var(--brand-soft);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
```

---

## 5. Páginas y Vistas — Especificaciones Detalladas

### 5.1 Vista: Home / Landing (`GET /`)

**Estado: No autenticado**
```
HeroSection
  ├── Título gradient: "La red social que piensa"
  ├── Subtítulo del PRD
  ├── CTA primario: "Crear cuenta gratis" → /register
  └── CTA secundario: "Iniciar sesión" → /login

NodeGrid (preview público, read-only)
  └── NodeCard × N
```

**Estado: Autenticado**
```
Dashboard
  ├── Saludo personalizado + rol + logout
  └── Panel de acciones rápidas

NodeCreateForm
  ├── Input: título
  ├── Input: descripción
  ├── Select: categoría
  └── Feedback Guardián IA (#guardian-feedback)

NodeGrid
  ├── "Mis Nodos" (filtro)
  └── "Todos los Nodos"
```

### 5.2 Vista: Detalle de Nodo (`GET /nodes/:id`)

```
NodeHeader
  ├── Breadcrumb: Inicio / {node.Title}
  ├── Título con gradient
  ├── Badges: categoría, ID corto, # resúmenes IA
  └── Descripción

TabBar
  ├── [Chat] ← activo por defecto
  ├── [Hilos]
  └── [Recursos] (P1)

NodeTimeline
  ├── ChatMessageItem × N
  └── AISummaryCard × N (intercalados cronológicamente)

ChatInputArea
  ├── Textarea (auto-expand, Enter = enviar)
  ├── Botón Enviar
  ├── Contador caracteres
  └── CronistaHint
```

### 5.3 Vista: Explorar Nodos (`GET /nodes`) [NUEVO]

```
PageHeader: "Explorar Nodos"

NodeSearchBar
  ├── Input: hx-get="/nodes/search"
  │         hx-trigger="keyup changed delay:300ms"
  │         hx-target="#node-results"
  └── Select: filtro por categoría

NodeResults (#node-results)
  └── NodeCard × N
```

### 5.4 Vista: Hilo de Foro (`GET /nodes/:id/threads/:tid`) [NUEVO]

```
ThreadHeader
  ├── Breadcrumb: Inicio / {node} / Hilos / {thread.Title}
  ├── Badge: [Resumen IA] si is_ai_generated
  ├── Título, autor, fecha
  └── Cuerpo (Markdown renderizado)

ReplyList (#replies-list)
  └── ReplyItem × N

ReplyForm
  └── Textarea + Enviar
      hx-post="/nodes/:id/threads/:tid/reply"
      hx-target="#replies-list"
      hx-swap="beforeend"
```

---

## 6. Estado de Autenticación y Condicionales de UI

### 6.1 Matriz de Visibilidad de Componentes

| Componente | No auth | Auth + No miembro | Auth + Miembro | Auth + Owner/Mod |
|---|---|---|---|---|
| NodeGrid | ✅ Read | ✅ Read | ✅ Read | ✅ Read |
| NodeCreateForm | ❌ Oculto | ✅ Visible | ✅ Visible | ✅ Visible |
| ChatInputArea | ❌ → CTA login | ❌ → CTA unirse | ✅ Visible | ✅ Visible |
| ReplyForm | ❌ → CTA login | ❌ → CTA unirse | ✅ Visible | ✅ Visible |
| BtnUnirse | ❌ | ✅ Visible | ❌ Oculto | ❌ Oculto |
| BtnSalir | ❌ | ❌ | ✅ Visible | ❌ |
| PanelModeración | ❌ | ❌ | ❌ | ✅ Visible |
| AuditNav | ❌ | ❌ | ❌ | Solo Admin |

### 6.2 Patrón de Auth en Templ

```go
// Patrón estándar para componentes condicionados por auth
templ ChatInputArea(nodeID string, isMember bool, isAuth bool) {
    <div class="chat-input-area">
        if !isAuth {
            @LoginCTA("Inicia sesión para participar")
        } else if !isMember {
            @JoinCTA(nodeID, "Únete al Nodo para chatear")
        } else {
            @ChatForm(nodeID)
        }
    </div>
}
```

---

## 7. Integración WebSocket — Plan de Progresión

### 7.1 Fase MVP: Polling HTMX

```html
<div id="chat-messages"
     hx-get="/nodes/{nodeID}/chat/poll"
     hx-trigger="every 3s"
     hx-swap="beforeend"
     hx-target="#chat-messages">
```

**Endpoint:** `GET /nodes/:id/chat/poll?after={last_msg_id}`
- Devuelve solo mensajes nuevos como fragmentos HTML
- Si no hay nuevos: devuelve `204 No Content` (HTMX no hace swap)

### 7.2 Fase Post-MVP: HTMX WebSocket Extension

```html
<div id="chat-messages"
     hx-ext="ws"
     ws-connect="/nodes/{nodeID}/ws">
```

```go
// Go handler escribe HTML directo al WS
conn.WriteMessage(websocket.TextMessage, []byte(
    `<div hx-swap-oob="beforeend:#chat-messages">` + fragment + `</div>`,
))
```

---

## 8. Organización de Archivos Estáticos

```
static/
├── css/
│   ├── design_system.css   # Variables CSS, reset, base
│   ├── components.css      # Buttons, cards, forms, badges
│   ├── layout.css          # Grid, sidebar, navbar, page-content
│   └── animations.css      # keyframes, transiciones
├── js/
│   ├── htmx.min.js         # (mover desde CDN)
│   ├── marked.min.js       # Markdown renderer (mover desde CDN)
│   └── nodal.js            # Lógica JS propia
└── img/
    └── logo.svg
```

**Migraciones prioritarias:**

| Archivo actual | Acción | Prioridad |
|---|---|---|
| CSS en `<style>` de `home.templ` | Mover a `design_system.css` + `components.css` | Alta |
| CSS en `<style>` de `node_detail.templ` | Mover a `components.css` + `layout.css` | Alta |
| CDN `htmx.org` | Mover a `static/js/` | Media |
| CDN `marked.min.js` | Mover a `static/js/` | Media |

---

## 9. Secuencia de Implementación Recomendada

### Fase A — Design System (Semana 1)
- [ ] Crear `static/css/design_system.css` con todos los tokens CSS
- [ ] Extraer `Layout()` a su propio archivo `layout.templ`
- [ ] Extraer `NavBar()` como componente independiente
- [ ] Mover CSS inline de `home.templ` a archivos estáticos
- [ ] Mover CDNs de htmx y marked a `static/js/`

### Fase B — Refactor de Vistas Existentes (Semana 2)
- [ ] Refactorizar `home.templ`: extraer `HeroSection`, `Dashboard`, `NodeCreateForm`, `NodeCard`
- [ ] Refactorizar `node_detail.templ`: extraer `NodeHeader`, `ChatMessageItem`, `AISummaryCard`, `ChatInputArea`
- [ ] Implementar fragmento `GuardianResponseFragment`
- [ ] Implementar condicionales de auth en `ChatInputArea`

### Fase C — Nuevas Vistas (Semanas 3-4)
- [ ] Crear `node_list.templ` con búsqueda HTMX
- [ ] Implementar `GET /nodes/search` endpoint con fragmento
- [ ] Crear `thread_view.templ` con `ThreadHeader`, `ReplyList`, `ReplyForm`
- [ ] Implementar endpoint de respuesta a hilos con fragmento

### Fase D — Layout Avanzado (Semana 5)
- [ ] Implementar Sidebar con lista de Nodos del usuario
- [ ] Implementar TabBar en vista de Nodo (Chat / Hilos / Recursos)
- [ ] Implementar polling HTMX para chat en tiempo real
- [ ] Implementar skeleton loaders con `hx-indicator`

### Fase E — Polish y UX (Semana 6)
- [ ] Revisar y unificar micro-animaciones
- [ ] Implementar empty states para todos los componentes
- [ ] Responsividad mobile (sidebar colapsable)
- [ ] Optimización de performance: preload, lazy images

---

## 10. Convenciones y Reglas de Desarrollo

### 10.1 Nomenclatura de Componentes Templ

```
Regla: PascalCase para funciones, kebab-case para clases CSS

✅ templ NodeCard(node database.Node)    → clase .node-card
✅ templ AISummaryCard(thread Thread)   → clase .ai-summary-card
❌ templ nodecard(...)                  → incorrecto: minúscula
❌ templ NodeCardComponent(...)         → incorrecto: sufijo redundante
```

### 10.2 Targets HTMX — IDs Semánticos

```
#chat-messages         → timeline de mensajes
#chat-counter          → contador de caracteres
#guardian-feedback     → respuesta del Agente Guardián
#node-results          → resultados de búsqueda de nodos
#replies-list          → lista de respuestas de hilo
#health-response       → respuesta del health check
#global-loader         → spinner HTMX global
```

### 10.3 Reglas de CSS

```
1. No usar !important en nuevos componentes
2. Todos los colores nuevos deben usar variables CSS (--var-name)
3. Transitions: ease o ease-in-out, máximo 300ms
4. z-index: sidebar=100, navbar=200, modals=300, tooltips=400
```

### 10.4 Reglas HTMX

```
1. Siempre definir hx-indicator para requests > 100ms esperados
2. Nunca usar hx-swap="outerHTML" en elementos con IDs de target
3. Para redirect post-submit: usar header HX-Redirect en Go
4. Los fragmentos NO incluyen <html>, <head> ni <body>
5. JS personalizado solo en nodal.js, no en atributos hx-on inline
   (excepción: lógica de una sola línea bien documentada)
```

---

## 11. Decisiones Arquitectónicas Pendientes (ADRs)

> [!IMPORTANT]
> Estas decisiones deben resolverse antes del inicio de la Fase D.

| # | Decisión | Opciones | Impacto |
|---|---|---|---|
| ADR-01 | ¿Sidebar fija o colapsable en mobile? | Fixed (Discord) vs Hamburger (Reddit) | Layout.templ, breakpoints |
| ADR-02 | ¿TabBar con URL change o sin él? | Tabs HTMX sin URL vs rutas `/nodes/:id/chat`, `/nodes/:id/threads` | Router Go, historial |
| ADR-03 | ¿WebSocket desde MVP o polling? | Polling 3s (simple) vs WS nativo (PRD) | node.go handler, NATS |
| ADR-04 | ¿Markdown SSR o CSR? | Go blackfriday (SSR) vs marked.js (CSR, actual) | Seguridad XSS, performance |
| ADR-05 | ¿Tailwind utility classes o CSS custom? | Tailwind CDN play vs CSS custom (producción, actual) | Consistencia, bundle size |

---

## 12. Glosario Frontend

| Término | Definición |
|---|---|
| **Fragmento** | Respuesta HTML parcial de un endpoint HTMX (sin Layout) |
| **Orquestador** | Componente Templ de nivel de página que ensambla sub-componentes |
| **Design Token** | Variable CSS que representa un valor semántico del design system |
| **hx-boost** | Modo HTMX que convierte navegación de `<a>` en AJAX automático |
| **hx-swap-oob** | "Out of Band Swap": HTMX actualiza múltiples targets en una respuesta |
| **Timeline** | Vista unificada que mezcla chat messages e AI summaries cronológicamente |
| **GuardianFeedback** | Fragmento HTML con la respuesta del Agente Guardián al crear un Nodo |

---

> **Fin del Plano Arquitectónico.** Este documento debe ser revisado y aprobado antes de iniciar la implementación. Los ADRs pendientes (Sección 11) deben resolverse antes de la Fase D. Todo código generado debe trazarse a una sección de este documento o del PRD_NODAL.md.
