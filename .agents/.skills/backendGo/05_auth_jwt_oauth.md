Rol: Desarrollador Go Senior experto en Arquitecturas Limpias, PostgreSQL, Templ y htmx.

Contexto:
El proyecto "Nodal" ya tiene su infraestructura base funcionando: PostgreSQL 16 con pgvector levantado en Docker, la estructura de carpetas del monorepo Go establecida, y el esquema SQL inicial desplegado (tablas users, nodes, node_memberships, etc. según el archivo docs/PRD_NODAL.md, Sección 5). El servidor HTTP nativo en Go ya está arriba y conectado a la base de datos.

Tarea Actual (Fase 2 — Auth: JWT + OAuth2 con RBAC):
Debes implementar el sistema completo de autenticación y gestión de sesiones de Nodal. El objetivo es que un usuario pueda registrarse, iniciar sesión (con email/contraseña o con OAuth2) y que cada request protegido valide su identidad y rol.

Por favor, genera el código en bloques explícitos para lo siguiente:

1. Hashing de Contraseñas (internal/platform/crypto/password.go):
   Implementa dos funciones: HashPassword(plain string) (string, error) y CheckPassword(plain, hash string) bool. Usa obligatoriamente Argon2id (golang.org/x/crypto/argon2) tal como exige el PRD en la Sección 9 (Seguridad).

2. Repositorio de Usuarios (internal/platform/database/user_repo.go):
   Crea el struct User (mapea a la tabla users del esquema SQL). Implementa las funciones:
   - CreateUser(db *sql.DB, username, email, passwordHash string) (*User, error)
   - FindUserByEmail(db *sql.DB, email string) (*User, error)
   - FindUserByID(db *sql.DB, id string) (*User, error)
   Usa database/sql sin ORM. Manejo de errores estricto (distinguir sql.ErrNoRows).

3. Lógica JWT (internal/auth/jwt.go):
   Usa la librería golang-jwt/jwt/v5. Implementa:
   - GenerateTokenPair(userID, role string) (accessToken string, refreshToken string, error): El access token debe expirar en 15 minutos y el refresh token en 7 días. Ambos deben incluir los claims: user_id, role y exp. La clave secreta debe leerse de una variable de entorno JWT_SECRET.
   - ValidateToken(tokenString string) (*Claims, error): valida firma y expiración.

4. Handlers de Autenticación (internal/handlers/auth.go):
   Crea los manejadores HTTP para:
   - POST /auth/register: recibe JSON con {username, email, password}, valida que el email no exista, hashea la contraseña y crea el usuario. Devuelve el token par.
   - POST /auth/login: recibe JSON con {email, password}, verifica credenciales, devuelve token par en JSON y setea el refresh token en una cookie HttpOnly Secure.
   - POST /auth/refresh: lee la cookie de refresh token, lo valida y emite un nuevo access token.
   - POST /auth/logout: elimina la cookie del refresh token.

5. Middleware de Autenticación y RBAC (internal/middleware/auth.go):
   Crea un middleware RequireAuth que extraiga el Bearer token del header Authorization, lo valide con ValidateToken e inyecte los claims en el contexto de la request (usando una clave de contexto tipada, no string). Crea además RequireRole(roles ...string) que devuelva 403 si el rol del usuario no está en la lista permitida.

6. Integración OAuth2 — Google y GitHub (internal/auth/oauth.go):
   Usa golang.org/x/oauth2 con los providers de Google y GitHub. Implementa el flujo Authorization Code:
   - GetGoogleAuthURL(state string) string y GetGitHubAuthURL(state string) string
   - HandleGoogleCallback(ctx context.Context, code string) (*User, error): intercambia el code, obtiene el email del usuario desde la API de Google, y aplica un "upsert" en la tabla users (si el email ya existe, devuelve el usuario; si no, lo crea con password_hash vacío). Lo mismo para HandleGitHubCallback.
   Los Client ID y Secret deben leerse de variables de entorno (GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET).

7. Gestión de Roles en BD (migrations/02_roles.sql):
   Asegúrate de que la columna role en node_memberships admita los valores 'owner', 'moderator', 'member' con un CHECK constraint. Genera la migración SQL para añadir este constraint de forma segura (IF NOT EXISTS).

8. Actualización de Rutas (cmd/nodal/main.go):
   Muéstrame solo las líneas exactas de mux.HandleFunc y la aplicación del middleware que debo añadir para registrar todas las nuevas rutas de auth y un ejemplo de ruta protegida (e.g., GET /nodes que requiera estar autenticado como mínimo con rol 'member').

Reglas:

1. Argon2id es obligatorio para el hashing. Prohibido bcrypt u otras alternativas.
2. Nunca loguees contraseñas, tokens completos ni datos sensibles de usuario. Usa logs estructurados (JSON).
3. Todos los tokens JWT se validan en cada request; no hay sesión de servidor (stateless).
4. El refresh token SOLO se transmite en cookie HttpOnly con flag Secure y SameSite=Strict.
5. El flujo OAuth2 debe proteger el parámetro state contra CSRF (genéralo aleatoriamente y valídalo en el callback).
6. Usa database/sql sin GORM ni ningún otro ORM.
7. Código modular: cada archivo tiene una única responsabilidad. Manejo de errores estricto en todas las funciones.
8. IMPORTANTE: Solo escribe los bloques de código en texto. NO intentes ejecutar comandos en tu entorno interno.
