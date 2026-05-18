# Documento de Requisitos de Producto (PRD) — Nodal

> **Versión:** 1.0.0 | **Fecha:** 18 de mayo de 2026 | **Estado:** Borrador Fundacional

---

## 1. Resumen Ejecutivo

**Nodal** es una red social **Topic-First** que organiza toda la interacción en torno a **Nodos**: comunidades temáticas con chat en vivo, hilos de foro y galería de recursos. Un ecosistema multi-agente de IA automatiza la gobernanza semántica, la síntesis de conocimiento y el enriquecimiento de metadatos.

---

## 2. Visión del Producto

*"Conectar personas a través de lo que les apasiona, no de quiénes son."*

### Problema

| Problema | Manifestación |
|---|---|
| Fragmentación | Mismos temas dispersos en Discord, Reddit, Telegram |
| Muro vacío | Experiencia fría sin contenido al registrarse |
| Duplicación semántica | Grupos idénticos compitiendo por audiencia |
| Pérdida de conocimiento | Chats efímeros sin síntesis |

### Propuesta de Valor

1. **Descubrimiento por interés**: buscar/crear un Nodo y acceder a comunidad activa.
2. **Gobernanza semántica autónoma**: IA previene duplicados y mantiene taxonomía limpia.
3. **Preservación de conocimiento**: resúmenes automáticos de conversaciones.
4. **Enriquecimiento contextual**: metadatos auto-completados vía APIs externas.

---

## 3. Revisión Arquitectónica — Observaciones Críticas

### 3.1 Redundancia en Capa Vectorial

**Propuesta original:** PostgreSQL + ChromaDB + Supabase.

**Problema:** ChromaDB y Supabase (vía pgvector) cumplen roles solapados.

**Resolución:** Consolidar en **PostgreSQL + pgvector**. Consultas híbridas (relacionales + vectoriales) en una misma transacción. Menor superficie de ataque. ChromaDB queda como alternativa solo si benchmarks demuestran que pgvector no satisface latencia con >1M embeddings.

### 3.2 Escalabilidad de WebSockets

**Riesgo:** Escalabilidad horizontal requiere sincronización entre instancias.

**Resolución:** Incorporar **NATS** como bus de mensajes inter-instancia para propagar eventos de chat entre nodos Go.

### 3.3 Definición del Servidor MCP

**Riesgo:** Término ambiguo.

**Resolución:** Se adopta **Model Context Protocol estándar** que expone tools a agentes IA. El Agente Curador consume tools para invocar APIs externas de forma estandarizada.

### 3.4 Moderación de Contenido

**Riesgo:** El Agente Guardián solo cubre creación de Nodos, no contenido interno.

**Resolución:** Diferir al post-MVP. Para MVP: reportes manuales con cola de revisión humana.

### 3.5 Autenticación

**Resolución:** JWT + refresh tokens. OAuth2 (Google, GitHub, Discord). RBAC con roles: `owner`, `moderator`, `member`.

---

## 4. Arquitectura del Sistema

### 4.1 Vista General

```
┌─────────────────────────────────────────────┐
│              USUARIOS (htmx)                │
└─────────────────┬───────────────────────────┘
                  │ HTTPS / WSS
                  ▼
┌─────────────────────────────────────────────┐
│         REVERSE PROXY (Caddy)               │
└────┬────────────────────────────────┬───────┘
     ▼                                ▼
┌──────────┐                    ┌──────────┐
│ Go Inst.A│◄────NATS Pub/Sub──►│ Go Inst.N│
└────┬─────┘                    └────┬─────┘
     │              ┌────────────────┘
     ▼              ▼
┌─────────────────────────────────────────────┐
│  PostgreSQL (Relacional + pgvector)         │
└─────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────┐
│     MICROSERVICIOS IA (Python + uv)         │
│  ┌─────────┐ ┌─────────┐ ┌──────────────┐  │
│  │Guardián │ │Cronista │ │Curador + MCP │  │
│  └─────────┘ └─────────┘ └──────────────┘  │
└─────────────────────────────────────────────┘
```

### 4.2 Stack Tecnológico

| Capa | Tecnología | Justificación |
|---|---|---|
| Frontend | Templ + htmx | Server-side rendering, SPA sin JS frameworks |
| Backend | Go 1.22+ | Goroutines para WebSockets masivos |
| BD Relacional | PostgreSQL 16+ | Madurez, JSONB, extensibilidad |
| BD Vectorial | pgvector | Embeddings integrados en PostgreSQL |
| Message Broker | NATS | Ultra-ligero, nativo Go, JetStream |
| IA | Python 3.12+ (uv) | Ecosistema ML maduro |
| MCP Server | FastMCP (Python) | Tools estandarizadas para agentes |
| Auth | JWT + OAuth2 | Stateless, escalable |
| Proxy | Caddy | HTTPS automático |
| CI/CD | GitHub Actions | Integrado con repositorio |

### 4.3 Comunicación Inter-Servicio

- **Backend ↔ Agentes**: gRPC (síncrono) o NATS (asíncrono).
- **Curador ↔ MCP Server**: JSON-RPC sobre stdio/SSE.
- **Chat real-time**: WebSocket nativo Go, propagado vía NATS entre instancias.

---

## 5. Arquitectura de Base de Datos

### 5.1 Capa 1 — Datos Relacionales

```sql
-- USUARIOS
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(30) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    avatar_url TEXT,
    bio TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- NODOS
CREATE TABLE nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(100) UNIQUE NOT NULL,
    title VARCHAR(150) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(50),
    metadata JSONB DEFAULT '{}',
    owner_id UUID REFERENCES users(id),
    member_count INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- MEMBRESÍAS
CREATE TABLE node_memberships (
    user_id UUID REFERENCES users(id),
    node_id UUID REFERENCES nodes(id),
    role VARCHAR(20) DEFAULT 'member',
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, node_id)
);

-- CHAT
CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID REFERENCES nodes(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_chat_node_time ON chat_messages(node_id, created_at DESC);

-- HILOS
CREATE TABLE threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID REFERENCES nodes(id) ON DELETE CASCADE,
    author_id UUID REFERENCES users(id),
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    is_ai_generated BOOLEAN DEFAULT FALSE,
    pinned BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- RESPUESTAS
CREATE TABLE thread_replies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    author_id UUID REFERENCES users(id),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- RECURSOS
CREATE TABLE resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID REFERENCES nodes(id) ON DELETE CASCADE,
    uploader_id UUID REFERENCES users(id),
    title VARCHAR(200) NOT NULL,
    url TEXT NOT NULL,
    type VARCHAR(20),
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 5.2 Capa 2 — Datos Vectoriales (pgvector)

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE node_embeddings (
    node_id UUID PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    embedding vector(384),
    model_version VARCHAR(50) DEFAULT 'all-MiniLM-L6-v2',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_node_embedding_hnsw
    ON node_embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 200);
```

### 5.3 Capa 3 — Auditoría de Agentes

```sql
CREATE TABLE agent_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_type VARCHAR(30) NOT NULL,
    action VARCHAR(50) NOT NULL,
    input_data JSONB,
    output_data JSONB,
    confidence FLOAT,
    node_id UUID REFERENCES nodes(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_audit_agent_time ON agent_audit_log(agent_type, created_at DESC);
```

---

## 6. Ecosistema Multi-Agente de IA

### 6.1 Principios de Diseño

1. **Autonomía acotada**: cada agente opera de forma independiente pero con límites definidos y auditoría completa.
2. **Transparencia**: toda decisión de IA se registra en `agent_audit_log` con datos de entrada, salida y confianza.
3. **Degradación elegante**: si un agente falla, el sistema funciona sin él (el usuario crea el Nodo manualmente, no hay resumen automático, los metadatos quedan vacíos).
4. **Umbral humano**: decisiones con confianza < 0.7 se escalan a moderación humana.

### 6.2 Agente Guardián (Clasificador Semántico)

**Misión:** Prevenir la proliferación de Nodos duplicados garantizando unicidad semántica en el grafo temático.

**Trigger:** Evento `node.creation.requested` emitido por el Backend Go vía NATS.

**Flujo:**

```
1. Usuario solicita crear Nodo "Skins Valorant"
2. Backend emite evento node.creation.requested
3. Agente Guardián recibe evento:
   a. Genera embedding del título + descripción (all-MiniLM-L6-v2)
   b. Consulta pgvector: SELECT nodos con similitud > 0.85
   c. Si hay coincidencias:
      - similitud > 0.95 → BLOQUEAR, redirigir al Nodo existente
      - similitud 0.85-0.95 → SUGERIR fusión, decisión del usuario
      - similitud < 0.85 → APROBAR creación
   d. Registra decisión en agent_audit_log
4. Backend recibe respuesta y actúa
```

**Skills del Agente:**

| Skill | Descripción |
|---|---|
| `generate_embedding` | Genera vector semántico del texto de entrada |
| `search_similar_nodes` | Consulta pgvector con umbral configurable |
| `evaluate_similarity` | Aplica lógica de decisión multi-umbral |
| `register_decision` | Persiste auditoría de la decisión |

**Rules:**
- R1: Nunca aprobar un Nodo con similitud > 0.95 sin intervención humana.
- R2: El embedding debe generarse con título + descripción concatenados.
- R3: Si pgvector no responde en 2s, aprobar con flag `needs_review=true`.
- R4: Máximo 5 candidatos similares retornados al usuario.

**Modelo ML:** `sentence-transformers/all-MiniLM-L6-v2` (384 dims, 80MB, latencia <50ms en CPU).

### 6.3 Agente Cronista (RAG — Retrieval-Augmented Generation)

**Misión:** Sintetizar las conversaciones diarias de cada Nodo en hilos de foro estructurados, preservando el conocimiento colectivo.

**Trigger:** Cron job diario (02:00 UTC) + opción manual por moderadores.

**Flujo:**

```
1. Cron trigger a las 02:00 UTC
2. Para cada Nodo con >20 mensajes en las últimas 24h:
   a. Recuperar mensajes del día (SQL query)
   b. Filtrar mensajes triviales (<5 palabras, emojis puros)
   c. Chunking: dividir en bloques de ~500 tokens
   d. Generar resumen con LLM (prompt estructurado)
   e. Crear hilo en tabla threads con is_ai_generated=true
   f. Registrar en agent_audit_log
3. Notificar a miembros del Nodo (opcional)
```

**Skills del Agente:**

| Skill | Descripción |
|---|---|
| `fetch_daily_messages` | Recupera mensajes del último periodo |
| `filter_noise` | Elimina mensajes triviales |
| `chunk_messages` | Divide mensajes en bloques procesables |
| `generate_summary` | Produce resumen estructurado vía LLM |
| `publish_thread` | Crea el hilo en la BD |

**Rules:**
- R1: Solo resumir Nodos con >20 mensajes/día (umbral configurable).
- R2: El resumen no debe exceder 800 palabras.
- R3: Incluir atribución anónima ("Un usuario mencionó que...").
- R4: No incluir contenido reportado o eliminado.
- R5: El hilo se publica con tag `[Resumen IA]` visible.

**Prompt Template:**

```
Eres el Cronista del Nodo "{node_title}". Tu tarea es generar un
resumen estructurado de la conversación del día.

REGLAS:
- Identifica 3-5 temas principales discutidos
- Destaca consensos y debates abiertos
- Menciona recursos compartidos (links, imágenes)
- Usa formato: ## Temas | ## Destacados | ## Recursos
- Tono: informativo, neutral, conciso
- NO incluir usernames reales

MENSAJES DEL DÍA:
{messages}
```

### 6.4 Agente Curador (Vía MCP Server)

**Misión:** Enriquecer automáticamente los metadatos de Nodos recién creados consultando APIs externas a través de un servidor MCP estandarizado.

**Trigger:** Evento `node.created` emitido tras aprobación del Agente Guardián.

**Flujo:**

```
1. Nodo "One Piece Vol. 105" creado y aprobado
2. Backend emite evento node.created
3. Agente Curador recibe evento:
   a. Analiza título + categoría del Nodo
   b. Selecciona tools MCP relevantes según categoría
   c. Invoca tools del MCP Server:
      - tool: manga_metadata → trae capítulos, autor, fecha
      - tool: cover_image → trae portada oficial
   d. Actualiza nodes.metadata (JSONB) con datos obtenidos
   e. Registra en agent_audit_log
4. Frontend muestra metadatos enriquecidos automáticamente
```

**Tools MCP Definidas (MVP):**

| Tool | Fuente | Categorías |
|---|---|---|
| `manga_metadata` | Jikan API (MyAnimeList) | Manga, Anime |
| `game_metadata` | RAWG API | Videojuegos |
| `movie_metadata` | TMDB API | Cine, Series |
| `music_metadata` | MusicBrainz | Música |
| `book_metadata` | Open Library API | Libros |
| `pokemon_metadata` | PokéAPI | Pokémon |
| `tech_metadata` | GitHub API | Tecnología, Open Source |

**Rules:**
- R1: Solo enriquecer si la categoría del Nodo mapea a una tool disponible.
- R2: No sobrescribir metadatos editados manualmente por el usuario.
- R3: Timeout de 10s por llamada API externa.
- R4: Si la API externa falla, el Nodo se crea sin metadatos (no bloquear).
- R5: Cachear respuestas de APIs externas por 24h.

**Ejemplo de MCP Server (FastMCP):**

```python
from fastmcp import FastMCP
import httpx

mcp = FastMCP("nodal-curator")

@mcp.tool()
async def manga_metadata(title: str) -> dict:
    """Obtiene metadatos de un manga desde Jikan API."""
    async with httpx.AsyncClient() as client:
        r = await client.get(
            f"https://api.jikan.moe/v4/manga",
            params={"q": title, "limit": 1}
        )
        data = r.json()["data"][0]
        return {
            "title_jp": data["title_japanese"],
            "chapters": data["chapters"],
            "author": data["authors"][0]["name"],
            "synopsis": data["synopsis"][:300],
            "cover_url": data["images"]["jpg"]["image_url"],
        }
```

---

## 7. Funcionalidades MVP

### 7.1 Alcance del MVP

| Funcionalidad | Prioridad | Descripción |
|---|---|---|
| **Registro/Login** | P0 | Email + OAuth2 (Google, GitHub) |
| **Explorar Nodos** | P0 | Búsqueda textual + por categoría |
| **Crear Nodo** | P0 | Con validación del Agente Guardián |
| **Unirse a Nodo** | P0 | Un clic, sin aprobación |
| **Chat en vivo** | P0 | WebSocket, historial persistente |
| **Hilos de Foro** | P0 | CRUD completo, respuestas anidadas (1 nivel) |
| **Galería de Recursos** | P1 | Subir links, imágenes (sin upload de archivos en MVP) |
| **Resúmenes IA** | P1 | Agente Cronista con cron diario |
| **Enriquecimiento** | P1 | Agente Curador para 3 categorías iniciales |
| **Perfil de usuario** | P1 | Avatar, bio, lista de Nodos unidos |
| **Reportes** | P2 | Reportar mensajes/hilos para moderación |
| **Notificaciones** | P2 | In-app, menciones en chat |

**Fuera del MVP:** Moderación IA intra-Nodo, sistema de reputación, PWA, mensajería directa, temas/skins personalizados.

### 7.2 User Stories MVP

**US-01: Crear Nodo con Validación Semántica**
```
COMO usuario registrado
QUIERO crear un Nodo sobre "Meta Competitivo Pokémon"
PARA encontrar otros jugadores y compartir estrategias

Criterios de Aceptación:
- El sistema valida que no exista un Nodo semánticamente equivalente
- Si existe uno similar (>85%), se muestra sugerencia de unirse
- Si es único, se crea el Nodo y se activa el Agente Curador
- El Nodo aparece en búsqueda inmediatamente
```

**US-02: Chat en Tiempo Real**
```
COMO miembro de un Nodo
QUIERO enviar y recibir mensajes en tiempo real
PARA participar en conversaciones de la comunidad

Criterios de Aceptación:
- Mensajes entregados en <200ms
- Historial de últimos 100 mensajes al entrar
- Scroll infinito para historial anterior
- Indicador de usuarios conectados
```

**US-03: Resumen Diario Automático**
```
COMO miembro de un Nodo activo
QUIERO leer un resumen de lo discutido ayer
PARA mantenerme al día sin leer todo el chat

Criterios de Aceptación:
- Resumen publicado como hilo fijado con tag [Resumen IA]
- Contiene 3-5 temas principales
- Generado solo si hubo >20 mensajes
- No incluye usernames reales
```

---

## 8. Métricas de Éxito

| Métrica | Objetivo MVP | Medición |
|---|---|---|
| Nodos creados / semana | >50 | SQL count |
| Tasa de duplicados bloqueados | >80% de intentos duplicados | agent_audit_log |
| Mensajes de chat / día | >500 | SQL count |
| Retención D7 | >30% | Cohorte de registro |
| Precisión del Guardián | >90% (validada por reportes) | Auditoría manual mensual |
| Tiempo de respuesta API p95 | <300ms | Métricas del reverse proxy |

---

## 9. Requisitos No Funcionales

| Requisito | Especificación |
|---|---|
| **Rendimiento** | Latencia p95 < 300ms para endpoints HTTP, <200ms para WS |
| **Disponibilidad** | 99.5% uptime (aceptable para MVP) |
| **Seguridad** | HTTPS obligatorio, contraseñas hasheadas con Argon2, CORS estricto |
| **Escalabilidad** | Soportar 1,000 usuarios concurrentes en chat |
| **Observabilidad** | Logs estructurados (JSON), métricas Prometheus, trazas OpenTelemetry |
| **Testing** | >80% cobertura en lógica de negocio, tests de integración para agentes |
| **Código** | Clean Code, SOLID, revisión de pares obligatoria |

---

## 10. Roadmap por Fases

### Fase 1 — Fundamentos (Semanas 1-4)
- [ ] Setup del monorepo y CI/CD
- [ ] PostgreSQL + pgvector en Docker
- [ ] Auth (JWT + OAuth2)
- [ ] CRUD de Nodos (sin IA)
- [ ] Perfil de usuario básico

### Fase 2 — Comunicación (Semanas 5-8)
- [ ] WebSocket chat en Go
- [ ] NATS para escalabilidad horizontal
- [ ] Hilos de foro CRUD
- [ ] Galería de recursos (links)

### Fase 3 — Inteligencia (Semanas 9-12)
- [ ] Agente Guardián + pgvector
- [ ] Agente Cronista + cron diario
- [ ] Agente Curador + MCP Server (3 tools iniciales)
- [ ] Dashboard de auditoría de agentes

### Fase 4 — Polish MVP (Semanas 13-14)
- [ ] Búsqueda de Nodos (full-text + semántica)
- [ ] Notificaciones in-app
- [ ] Sistema de reportes
- [ ] Testing end-to-end
- [ ] Deploy en producción

---

## 11. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|---|---|---|---|
| pgvector no escala con >1M embeddings | Media | Alto | Benchmark temprano; ChromaDB como plan B |
| APIs externas del Curador cambian/caen | Alta | Medio | Circuit breaker, cache 24h, fallback graceful |
| LLM genera resúmenes incorrectos | Media | Medio | Prompt engineering iterativo, flag manual |
| Abuso de creación de Nodos (spam) | Media | Alto | Rate limiting + captcha + umbral del Guardián |
| Complejidad de mantener 3 agentes | Media | Medio | Observabilidad exhaustiva, runbooks |

---

## 12. Glosario

| Término | Definición |
|---|---|
| **Nodo** | Comunidad temática. Unidad organizativa central de Nodal. |
| **Hub** | Agrupación de Nodos relacionados (post-MVP). |
| **Agente Guardián** | Microservicio IA que valida unicidad semántica de Nodos. |
| **Agente Cronista** | Microservicio IA que genera resúmenes diarios vía RAG. |
| **Agente Curador** | Microservicio IA que enriquece metadatos vía MCP Server. |
| **MCP** | Model Context Protocol. Estándar para exponer tools a agentes IA. |
| **Embedding** | Representación vectorial de texto para comparación semántica. |
| **pgvector** | Extensión de PostgreSQL para almacenar y consultar vectores. |
| **NATS** | Message broker ligero para comunicación entre servicios. |
| **XP** | Extreme Programming. Metodología ágil con énfasis en calidad. |

---

> **Fin del documento.** Este PRD constituye la especificación fundacional de Nodal v1.0 (MVP). Toda decisión de implementación debe trazarse a una sección de este documento.
