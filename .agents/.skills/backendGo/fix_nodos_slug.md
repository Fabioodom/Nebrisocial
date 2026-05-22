Rol: Desarrollador Go Senior experto en Arquitecturas Limpias, htmx y PostgreSQL.

Contexto del problema: > Actualmente el formulario de creación de Nodos de nuestra aplicación falla porque hay un desajuste entre el esquema SQL de la base de datos y la capa de repositorios de Go.
Nuestro init.sql define la tabla nodes con los campos obligatorios: slug (VARCHAR UNIQUE NOT NULL), title (VARCHAR NOT NULL) y description (TEXT).
Sin embargo, el flujo actual del frontend (home.templ), el controlador (node.go) y el repositorio (node_repo.go) están usando la variable name e ignorando el slug, lo que provoca un crash en PostgreSQL durante el INSERT.

Tarea:
Necesito que reescribas y alinees el flujo completo. Por favor, devuélveme el código completo y actualizado para estos 3 archivos:

1. Repositorio (internal/platform/database/node_repo.go):

Modifica CreateNode para que acepte title y description.

Añade lógica interna súper sencilla para auto-generar un slug a partir del title (ej. pasarlo a minúsculas, quitar caracteres raros y cambiar espacios por guiones usando strings.ReplaceAll).

Actualiza la query SQL: INSERT INTO nodes (slug, title, description) VALUES ($1, $2, $3)

2. Controlador HTTP (internal/handlers/node.go):

Ajusta el manejador para que extraiga r.FormValue("title") en lugar de name.

Añade manejo de errores para que, si el db.Exec falla, imprima el error real de PostgreSQL en el <div> de error (usando err.Error()), así no falla de forma silenciosa.

3. Vista Frontend (internal/handlers/views/home.templ):

Cambia los atributos id y name del input del formulario de "name" a "title".

Reglas:
Devuelve solo los bloques de código listos para reemplazar los archivos. Asegúrate de incluir los imports necesarios (como "strings" en el repo).