"""
prompts.py — Templates de prompt para el Agente Investigador de Frontends.

Todos los prompts están definidos como constantes de string con placeholders
{variable} para formateo con str.format(). Separar prompts del código Python
facilita la iteración rápida sobre el wording sin tocar la lógica del generador.
"""

# ─────────────────────────────────────────────────────────────────────────────
# SYSTEM PROMPT
# Establece el rol, restricciones absolutas y contexto del LLM.
# Se envía como SystemMessage en cada invocación al LLM.
# ─────────────────────────────────────────────────────────────────────────────

SYSTEM_PROMPT = """Eres un ingeniero experto en desarrollo frontend para el proyecto "Nodal",
especializado en Go Templ, HTMX y Tailwind CSS. Tu única función es generar
código de componentes y páginas UI en formato .templ para este proyecto.

## Tu stack exclusivo
- **Plantillas:** Go Templ (sintaxis templ, NO html/template de Go)
- **Estilos:** Tailwind CSS usando EXCLUSIVAMENTE las clases del design system de Nodal
- **Interactividad:** HTMX mediante atributos hx-* (ÚNICO mecanismo permitido)
- **Hiperscript:** Solo para micro-comportamientos del lado cliente que no requieren server round-trip

## Reglas ABSOLUTAS e INNEGOCIABLES

### 🚫 PROHIBIDO
1. Usar Alpine.js, React, Vue, Svelte, Angular, jQuery, Stimulus o CUALQUIER framework JS
2. Escribir código JavaScript inline (salvo _hyperscript en casos mínimos)
3. Usar fetch(), XMLHttpRequest, addEventListener() o Event Listeners JS
4. Usar colores Tailwind por defecto: bg-white, bg-gray-*, text-black, bg-blue-*, etc.
5. Usar html/template o text/template de Go en lugar de la sintaxis Go Templ
6. Generar clases CSS ad-hoc con style="" inline (excepto para variables CSS dinámicas)
7. Dejar imports no utilizados en el archivo .templ

### ✅ OBLIGATORIO
1. Dark Mode siempre: TODAS las clases de color provienen del design system Nodal (nodal-bg-*, nodal-surface-*, nodal-primary-*, nodal-text-*, etc.)
2. Todo comportamiento asíncrono usa atributos hx- (hx-get, hx-post, hx-trigger, hx-target, hx-swap)
3. hx-target siempre explícito (nunca omitido)
4. hx-swap con valores permitidos: innerHTML | outerHTML | beforeend
5. Accesibilidad mínima: aria-label en botones de icono, role="status" en spinners, alt en imágenes
6. Sintaxis Go Templ válida: package declaration, imports correctos, indentación con TABS
7. El código generado debe compilar sin errores con el compilador templ

## Design System Nodal
El design system completo está disponible en el contexto de cada prompt.
Léelo antes de generar cualquier componente. Es la única fuente de verdad para clases CSS.
"""

# ─────────────────────────────────────────────────────────────────────────────
# COMPONENT GENERATION PROMPT
# Prompt principal para generar un componente atómico o molecular.
# Placeholders: {component_description}, {design_system_context}, {existing_components}
# ─────────────────────────────────────────────────────────────────────────────

COMPONENT_GENERATION_PROMPT = """## Design System de Referencia

{design_system_context}

---

## Componentes Ya Existentes en el Proyecto

Los siguientes componentes Templ ya están implementados. Puedes usarlos mediante
composición (@NombreComponente(args)) en lugar de reimplementarlos:

{existing_components}

---

## Solicitud de Generación

Necesito que generes el siguiente componente UI:

**Descripción:** {component_description}

## Instrucciones de Generación

1. **Analiza** la descripción y determina:
   - Nombre del componente en PascalCase (ej: NodeCard, ChatInput, UserAvatar)
   - Props necesarias con sus tipos Go (string, int, bool, []string, etc.)
   - Dependencias HTMX: qué endpoints llamará y con qué atributos hx-
   - Si puede usar componentes existentes del proyecto mediante composición

2. **Genera** el archivo .templ completo con:
   - Bloque de comentario inicial con metadatos (formato exacto abajo)
   - Package declaration correcto (`package components` para atómicos, `package pages` para páginas)
   - Imports necesarios (solo los usados)
   - Función(es) Templ bien formadas con props tipadas
   - Clases Tailwind EXCLUSIVAMENTE del design system Nodal
   - Atributos HTMX donde corresponda
   - Atributos de accesibilidad mínimos

3. **Formato del bloque de comentario inicial:**
```
// Componente: NombreComponente
// Descripción: Una línea describiendo qué hace este componente.
// Props:
//   - propA (string): descripción de propA
//   - propB (int): descripción de propB
// HTMX:
//   - hx-get="/ruta" → carga X en #target-id
//   - hx-post="/ruta" → envía form y actualiza #target-id
// Accesibilidad: aria-label en [elemento], role="status" en [elemento]
```

4. **Devuelve SOLAMENTE** el bloque de código .templ entre marcadores:
```templ
[código go templ aquí]
```

No incluyas explicaciones antes o después del bloque de código.
El código debe estar listo para guardarse directamente como archivo .templ.
"""

# ─────────────────────────────────────────────────────────────────────────────
# PAGE GENERATION PROMPT
# Variante para generar una página completa (layout + secciones).
# Placeholders: {page_description}, {design_system_context}, {existing_components},
#               {page_sections}, {route_path}
# ─────────────────────────────────────────────────────────────────────────────

PAGE_GENERATION_PROMPT = """## Design System de Referencia

{design_system_context}

---

## Componentes Ya Existentes en el Proyecto

{existing_components}

---

## Solicitud de Generación de Página

Necesito que generes una página completa para la siguiente ruta:

**Ruta HTTP:** `{route_path}`
**Descripción:** {page_description}
**Secciones a incluir:**
{page_sections}

## Instrucciones de Generación de Página

1. **Estructura:** La página usa el componente `Layout` existente como wrapper.
   El `Layout` acepta el título de la página. Usa `@components.Layout(title)` para envolverla.

2. **Organización en archivo:**
   - Package: `package pages`
   - Imports: `"github.com/tuusuario/nodal/web/components"` y lo que necesites
   - Una función Templ principal para la página (PascalCase + "Page", ej: `NodeDetailPage`)
   - Sub-componentes privados de la página como funciones Templ adicionales en el mismo archivo

3. **Datos:**
   - Define un struct Go `NombrePáginaData` inline en el comentario si necesitas múltiples props
   - Las props del struct son las que el handler Go pasa al template
   - Usa `for _, item := range items { }` para listas dinámicas

4. **HTMX en páginas:**
   - Las secciones que cargan datos dinámicamente usan `hx-get` + `hx-trigger="load"`
   - La navegación entre páginas usa `hx-boost="true"` en el layout o `hx-push-url="true"`
   - Los formularios de la página usan `hx-post` directamente en el `<form>`

5. **Devuelve SOLAMENTE** el bloque de código .templ:
```templ
[código go templ completo de la página aquí]
```
"""

# ─────────────────────────────────────────────────────────────────────────────
# REVIEW PROMPT
# Segundo paso obligatorio: el LLM auto-revisa el código generado.
# Placeholders: {generated_code}, {design_system_context}
# ─────────────────────────────────────────────────────────────────────────────

REVIEW_PROMPT = """## Código Generado para Revisión

El siguiente código Go Templ fue generado en el paso anterior. Tu tarea es
revisarlo exhaustivamente y devolver la versión corregida si hay errores.

```templ
{generated_code}
```

## Checklist de Revisión Obligatorio

Verifica CADA punto de esta lista. Para cada ítem, indica ✅ (correcto) o ❌ (problema encontrado):

### Sintaxis Go Templ
- [ ] ¿El archivo tiene `package` declaration como primera línea?
- [ ] ¿Los imports están correctamente formateados y todos se usan?
- [ ] ¿La firma de la función Templ es correcta (`templ NombreFuncion(props tipos) {{`)?
- [ ] ¿La indentación usa TABS (no espacios)?
- [ ] ¿Las expresiones de interpolación usan `{{ variable }}` correctamente?
- [ ] ¿Los `for` loops y `if` condicionales tienen la sintaxis Go correcta?
- [ ] ¿Los componentes anidados se llaman con `@NombreComponente(args)`?

### Design System — Dark Mode
- [ ] ¿TODAS las clases de color son del design system nodal-* (nodal-bg-*, nodal-surface-*, nodal-text-*, nodal-primary-*, etc.)?
- [ ] ¿No hay ninguna clase Tailwind de color por defecto (bg-white, bg-gray-*, text-black, bg-blue-*, bg-red-*, etc.)?
- [ ] ¿Los estados hover usan hover:bg-nodal-* o hover:text-nodal-*?
- [ ] ¿Los bordes usan border-nodal-border o border-nodal-border-subtle?

### HTMX
- [ ] ¿hx-target está siempre especificado explícitamente?
- [ ] ¿Los valores de hx-swap son solo: innerHTML, outerHTML, o beforeend?
- [ ] ¿No hay JavaScript inline (fetch, addEventListener, etc.)?
- [ ] ¿No hay Alpine.js, React, Vue u otros frameworks JS?

### Accesibilidad
- [ ] ¿Los botones de solo icono tienen aria-label?
- [ ] ¿Los spinners/skeletons tienen role="status" y aria-label?
- [ ] ¿Las imágenes tienen atributo alt?
- [ ] ¿Los inputs de formulario tienen label asociado o aria-label?

## Instrucciones de Respuesta

Si encuentras problemas (❌), corrígelos en el código.
Si todo está correcto (todos ✅), devuelve el código tal cual.

**Devuelve SIEMPRE:**
1. El checklist completado (una línea por ítem con ✅ o ❌)
2. El código final corregido entre marcadores:
```templ
[código go templ final aquí]
```
3. Una línea con `REVIEW_NOTES:` seguida de un resumen de los cambios realizados
   (o "Sin cambios necesarios." si no hubo correcciones).
"""
