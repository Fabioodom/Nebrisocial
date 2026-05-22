Rol: Desarrollador Backend Senior experto en Go y Arquitecturas Limpias.

Contexto: Ya tenemos levantada la infraestructura de "Nodal" (Docker con PostgreSQL 16 + pgvector) y la estructura de carpetas.

Tarea Actual (Fase 1 - Conexión y Arranque):
Necesito que escribas el código para inicializar el servidor y conectarlo a la base de datos. Por favor, dame los comandos y el código en bloques explícitos para lo siguiente:

Inicialización del módulo: El comando exacto para inicializar el proyecto en Go (go mod init ...) y el comando para instalar el driver de PostgreSQL.

Conexión a la BD (internal/platform/database/db.go): Crea un archivo robusto que gestione el pool de conexiones a PostgreSQL usando la librería estándar database/sql y el driver pq. Haz que la cadena de conexión lea las variables de entorno (usuario, contraseña, bd).

Punto de entrada (cmd/nodal/main.go): Crea el archivo principal que arranque la aplicación. Debe inicializar la conexión a la base de datos de forma segura, levantar un servidor HTTP nativo en el puerto 8080 y exponer una única ruta /health que devuelva un simple texto "Nodal OK - DB Connected" si todo está bien.

Reglas:

Nada de ORMs pesados como GORM. Usa la librería estándar.

Manejo de errores estricto (si la base de datos no conecta, el servidor debe hacer un log.Fatal de forma elegante).

Mantén el código modular.