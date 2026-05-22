Rol: Desarrollador Full-Stack Senior experto en Go, PostgreSQL, Templ y htmx. Trabajas bajo Arquitecturas Limpias.

Contexto: Nuestro servidor Go (nodal) está conectado a PostgreSQL y ya renderiza la vista inicial con Templ y htmx. Ahora vamos a implementar la creación de la entidad principal de nuestra red social: el "Nodo" (una comunidad temática).

Tarea Actual (Fase 2 - Dominio Core: Nodos):
Necesito que me des el código en bloques explícitos para implementar la creación de Nodos de principio a fin. Proporciona lo siguiente:

1. SQL DDL (migrations/01_create_nodes.sql): El código SQL estándar para crear la tabla nodes. Debe tener: id (UUID o serial), name (varchar único), description (text), y created_at (timestamp).
2. Repositorio (internal/platform/database/node_repo.go): Crea el modelo Go (Node struct) y una función CreateNode(db *sql.DB, name, description string) error que ejecute el INSERT en la base de datos de forma segura.
3. Controlador HTTP (internal/handlers/node.go): Un manejador que reciba una petición POST desde un formulario, extraiga los valores name y description, llame a CreateNode, y devuelva un fragmento HTML simple (ej. <div class="success">Nodo creado con éxito</div>) para que htmx lo inyecte.
4. Actualización de la Vista (internal/handlers/views/home.templ): Modifica nuestro componente Home actual para añadir debajo del botón de Health Check un formulario simple. El formulario debe tener dos inputs (name, description), un botón de submit, y usar htmx (hx-post="/nodes", hx-target="#form-result") para enviar los datos al backend sin recargar la página.
5. Enrutador (cmd/nodal/main.go): Solo muéstrame la línea exacta de mux.HandleFunc que debo añadir para registrar la ruta POST /nodes con el nuevo controlador.

Reglas:

1. Usa la librería estándar database/sql (sin GORM).
2. Código modular y manejo de errores estricto.
3. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.