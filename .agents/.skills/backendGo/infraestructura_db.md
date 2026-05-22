Rol: Eres un Desarrollador Backend Senior experto en Go y Arquitecturas Limpias. Trabajas bajo la metodología Extreme Programming (XP) aplicando principios SOLID.

Contexto:
Te adjunto el Documento de Requisitos de Producto (PRD) llamado PRD_NODAL.md dentro de la carpeta docs nuestro proyecto "Nodal". Debes basar todo tu desarrollo estrictamente en este documento.

Tarea Actual (Fase 1 - Fundamentos):
Vamos a iniciar el desarrollo por la base. No generes la lógica de la aplicación todavía. Tu objetivo en este primer paso es preparar el entorno de infraestructura y la base de datos.

Por favor, genera lo siguiente:

1. Estructura del Proyecto Go: Define cómo debe ser la estructura de carpetas del monorepo (siguiendo los estándares de Go, separando cmd, internal, pkg, etc.).
2. Infraestructura (Docker): Crea un archivo docker-compose.yml robusto que levante una instancia de PostgreSQL 16 con la extensión pgvector instalada, lista para almacenar los embeddings de nuestros agentes de IA.

Esquema SQL (Init): Basándote en la "Sección 5. Arquitectura de Base de Datos" del PRD, genera un archivo init.sql que contenga la creación de las tablas relacionales (users, nodes, etc.), la tabla vectorial y la tabla de auditoría de agentes.

Reglas:

No uses frameworks web pesados en Go (usaremos la librería estándar o enrutadores ligeros como Chi/Mux más adelante).

Asegúrate de que el archivo docker-compose.yml exponga los puertos correctos y tenga variables de entorno seguras para la base de datos.

Explica brevemente cómo levantar este entorno.