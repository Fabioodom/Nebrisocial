# 🕸️ Nodal — Red Social Topic-First Impulsada por IA

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/Python-3.12%2B-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://www.python.org/)
[![NATS](https://img.shields.io/badge/NATS-Message%20Broker-808080?style=for-the-badge&logo=nats&logoColor=white)](https://nats.io/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16%2B-336791?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![HTMX](https://img.shields.io/badge/HTMX-SPA%20SSR-3D5F7F?style=for-the-badge&logo=htmx&logoColor=white)](https://htmx.org/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind%20CSS-v4.0-38B2AC?style=for-the-badge&logo=tailwind-css&logoColor=white)](https://tailwindcss.com/)
[![Docker](https://img.shields.io/badge/Docker-Infrastructure-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)

**Nodal** es una red social de alta disponibilidad diseñada bajo la filosofía **"Topic-First"**. Su objetivo es conectar a las personas en torno a sus intereses y pasiones (organizados en comunidades denominadas **Nodos**) en lugar de centrarse en perfiles individuales. Toda la plataforma está impulsada y dinamizada por un ecosistema multi-agente de Inteligencia Artificial que automatiza la gobernanza, la moderación semántica, el enriquecimiento contextual y la síntesis de información en tiempo real.

---

## 🛠️ Stack Tecnológico

El proyecto está diseñado bajo un modelo híbrido de alto rendimiento, optimizando el consumo de recursos tanto en el servidor web como en los microservicios de IA:

| Capa | Tecnologías | Propósito y Beneficios |
| :--- | :--- | :--- |
| **Backend** | `Go 1.22+`, `NATS` | Servidor web ultra-ligero y concurrente. NATS actúa como bus de mensajes de alta velocidad para la propagación de eventos inter-instancia (WebSockets). |
| **Frontend** | `Go Templ`, `HTMX`, `Tailwind CSS` | Server-Side Rendering (SSR) eficiente que proporciona una experiencia de Single Page Application (SPA) fluida sin la sobrecarga de frameworks JS pesados. |
| **Base de Datos** | `PostgreSQL 16`, `pgvector` | Consolidación de datos relacionales y almacenamiento de embeddings vectoriales en un único motor. Uso de CTEs para optimización y scoring de búsquedas híbridas. |
| **Ecosistema IA** | `Python 3.12`, `uv`, `LangChain`, `FastMCP` | Microservicios de agentes inteligentes con empaquetado optimizado mediante `uv` y comunicación estandarizada a través de herramientas de Model Context Protocol (MCP). |

---

## 🧠 Ecosistema Multi-Agente (La Joya de la Corona)

Nodal cuenta con una arquitectura de IA descentralizada en la que cooperan tres agentes especializados. Cada decisión importante tomada por la IA queda registrada de manera transparente en la tabla de base de datos `agent_audit_log`.

```
                        [ Nodos & Contenido ]
                                  │
      ┌───────────────────────────┼───────────────────────────┐
      ▼                           ▼                           ▼
🛡️ Agente Guardián          📜 Agente Cronista           🔮 Agente Curador
(Unicidad Semántica)         (RAG / Síntesis)             (Metadatos y MCP)
      │                           │                           │
  pgvector &                  Cron diario &                FastMCP &
  Similarity Check            LLM Summaries                APIs Externas
```

### 1. 🛡️ Agente Guardián (Clasificador Semántico)
* **Misión:** Garantiza la unicidad y el orden temático en la red social previniendo la proliferación de Nodos duplicados o redundantes.
* **Mecanismo:** Al solicitar la creación de un nuevo Nodo, genera un vector semántico (`all-MiniLM-L6-v2`) con el título y la descripción, comparándolo con la base de datos usando `pgvector`:
  * **Similitud > 0.95:** Creación bloqueada. Se redirige al usuario al Nodo existente de manera proactiva.
  * **Similitud entre 0.85 y 0.95:** Se sugiere la fusión con Nodos existentes o se escala a revisión humana.
  * **Similitud < 0.85:** Creación aprobada automáticamente.

### 2. 📜 Agente Cronista (RAG & Síntesis Diaria)
* **Misión:** Transforma el flujo caótico de los chats diarios en conocimiento estructurado, permitiendo a los usuarios ponerse al día rápidamente.
* **Mecanismo:** Un cron job programado recupera y limpia las conversaciones de las últimas 24 horas (en Nodos con >20 mensajes), eliminando ruidos y trivialidades. Aplica segmentación y compresión mediante LLM (vía prompts parametrizados) y publica automáticamente un hilo resumido con la etiqueta `[Resumen IA]` detallando temas clave, consensos y recursos compartidos de forma anónima.

### 3. 🔮 Agente Curador (Enriquecedor de Contexto vía MCP)
* **Misión:** Enriquece automáticamente el contenido de los Nodos recién aprobados trayendo información estructurada de fuentes externas.
* **Mecanismo:** Utiliza **FastMCP** en Python para interactuar con herramientas estandarizadas de Model Context Protocol (MCP). Dependiendo de la categoría asignada al Nodo, llama a las herramientas asociadas (por ejemplo, `manga_metadata` vía Jikan, `movie_metadata` vía TMDB o `tech_metadata` vía GitHub API) para autocompletar portadas, descripciones formales, creadores y estadísticas en los metadatos JSONB del nodo.

---

## 📐 Arquitectura y Funcionalidades Clave

* **💬 Chat en Tiempo Real:** Implementado con WebSockets nativos en Go. Gracias al bus de comunicación **NATS**, el chat escala horizontalmente de manera transparente propagando los mensajes instantáneamente entre diferentes réplicas del servidor.
* **🧵 Respuestas Anidadas (OOB HTMX):** Un sistema de foros dinámico que permite respuestas a hilos y actualizaciones asíncronas directas utilizando intercambio fuera de banda (*Out-of-Band Swaps*) de HTMX, reduciendo el consumo de ancho de banda y eliminando la necesidad de JavaScript reactivo en el cliente.
* **🔍 Buscador Global Unificado:**
  * Permite la búsqueda integrada tanto de **Nodos** (usando la similitud vectorial de `pgvector` e índices HNSW en PostgreSQL) como de **Usuarios** en una sola vista dinámica (`/explore`).
  * Implementa lógica SQL segura ante valores nulos utilizando `COALESCE` en campos opcionales (como `bio` o `avatar_url`) para blindar el `rows.Scan()` y la función `REPLACE` en base de datos.
  * Sincroniza dinámicamente el valor de la barra de búsqueda con la URL del navegador (`?q=...`) al cargar la página y después de los swaps de HTMX (`htmx:afterSwap`), sin interrumpir la escritura activa.
  * Ofrece el atajo de teclado **`Ctrl+K` / `Cmd+K`** para enfocar y seleccionar automáticamente todo el texto del buscador principal en un microsegundo.
* **🔔 Bandeja de Notificaciones Reactiva:** Panel in-app que rastrea menciones, nuevas respuestas e invitaciones en tiempo real con un badge dinámico de conteo de no leídas y modal de visualización detallada asíncrona.
* **📈 Feed Inteligente con Decaimiento Temporal (Time Decay):** Algoritmo de ordenamiento personalizado que prioriza posts e hilos populares basándose en votos, comentarios, actividad de agentes e introduciendo un factor físico de gravedad temporal para mantener el feed fresco y relevante.

---

## 📂 Estructura del Proyecto

```
.
├── cmd/                  # Puntos de entrada de la aplicación
│   └── nodal/            # Servidor web principal en Go
├── internal/             # Código interno de la aplicación Go
│   ├── auth/             # Autenticación JWT y OAuth2
│   ├── handlers/         # Controladores HTTP y plantillas de renderizado
│   │   └── views/        # Plantillas Templ (Layout, Home, Explore, etc.)
│   └── platform/         # Conectores de base de datos e infraestructura
│       └── database/     # Repositorios SQL y lógica de pgvector
├── migrations/           # Scripts de inicialización y evolución de la base de datos SQL
├── services/             # Microservicios auxiliares de Inteligencia Artificial (Python)
│   ├── cronista/         # Cron job de RAG y resúmenes diarios
│   ├── curator/          # Servidor MCP y herramientas de enriquecimiento
│   └── guardian/         # Clasificación semántica de duplicados
├── static/               # Recursos estáticos del frontend (CSS, JS, Imágenes)
│   └── css/              # Hojas de estilo del Design System y componentes
└── docker-compose.yml    # Orquestación de contenedores (PostgreSQL, NATS, etc.)
```

---

## 🚀 Instalación y Configuración

Sigue estos pasos detallados para configurar y ejecutar Nodal en tu entorno de desarrollo local.

### Prerrequisitos
Asegúrate de tener instalados los siguientes componentes:
* **Docker y Docker Compose** (para la base de datos PostgreSQL, pgvector y NATS)
* **Go 1.22+**
* **Python 3.12+** (junto con la herramienta de gestión de paquetes [uv](https://github.com/astral-sh/uv))
* **Templ CLI** (compilador de plantillas Go). Puedes instalarlo ejecutando:
  ```bash
  go install github.com/a-h/templ/cmd/templ@latest
  ```

---

### Pasos de Configuración

#### 1. Clonar el repositorio y configurar variables de entorno
Crea una copia local del archivo de configuración de entorno:
```bash
cp .env.example .env
```
Abre el archivo `.env` en tu editor de texto y configura las credenciales de base de datos, puertos, secretos de JWT y claves de cliente OAuth2 (para login con Google o GitHub) según corresponda.

#### 2. Levantar la infraestructura base
Utiliza Docker Compose para iniciar PostgreSQL (con la extensión `pgvector` activada), el broker NATS y los microservicios de los agentes de IA:
```bash
docker-compose up -d
```
> [!NOTE]
> La primera ejecución compilará las imágenes de Docker de los servicios Python e inicializará el esquema SQL base contenido en `migrations/init.sql`.

#### 3. Compilar el frontend (Templ)
Genera el código de Go correspondiente a las vistas diseñadas en los archivos `.templ`:
```bash
templ generate
```

#### 4. Iniciar el servidor web de Go
Arranca la aplicación principal en Go:
```bash
go run ./cmd/nodal/main.go
```

El servidor web estará disponible en [http://localhost:8080](http://localhost:8080).

---

## 🛡️ Licencia y Autores

Este proyecto está bajo la licencia MIT.

Desarrollado con ❤️ por el equipo de ingeniería de Nodal.

---
*Nodal: Conectando personas a través de lo que les apasiona.*
