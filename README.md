# Asesoría

[![CI](https://github.com/Xumeiquer/Asesoria/actions/workflows/ci.yml/badge.svg)](https://github.com/Xumeiquer/Asesoria/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/Xumeiquer/Asesoria?label=release)](https://github.com/Xumeiquer/Asesoria/releases)

ERP fiscal local para autónomos y pequeñas sociedades limitadas. Organiza las obligaciones de la AEAT, la TGSS y el Registro Mercantil, genera el calendario de vencimientos, guarda documentos y ayuda a preparar los modelos oficiales. Una extensión de navegador muestra los datos del expediente sobre la sede electrónica y permite copiarlos o descargarlos, pero **nunca firma ni accede al certificado**.

La aplicación está pensada para un único usuario en su propio equipo: el backend escucha solo en `127.0.0.1`, los datos viven en SQLite y no hay telemetría ni servicios externos.

## Estado del proyecto

MVP funcional. Incluye:

- Catálogo declarativo de **38 modelos** (AEAT, TGSS y Registro Mercantil) con periodicidad y reglas de aplicabilidad por perfil fiscal.
- Generación de calendario por ejercicio, con traslado de vencimientos de fin de semana al lunes.
- Expediente por obligación: estado, notas, checklist, documentos y justificante.
- Cálculo automático de **modelo 303 (IVA)** y **modelo 130 (pago fraccionado de IRPF)** a partir de facturas registradas.
- Gestión de contribuyentes (autónomo o SL), facturas emitidas/recibidas y documentos content-addressed por SHA-256.
- Perfil RETA por autónomo: previsión anual de rendimientos netos, base de cotización mensual con fecha de vigencia e historial de cambios. La base se valida contra la tabla general y reducida de 2026; no cubre particularidades como pluriactividad o actividades artísticas.
- Biblioteca documental gestionada en una carpeta del equipo, con árbol por contribuyente/año/trimestre/tipo, escaneo incremental, extracción de texto y OCR local opcional.
- Extensión MV3 para Chrome/Chromium: emparejamiento por código de un solo uso, próximos vencimientos, panel sobre la sede electrónica con casillas, copia al portapapeles y descarga de documentos.
- Datos de demostración con `-demo`.

Todavía **no** presenta modelos, no custodia certificados y no emite facturas. El resto de modelos del catálogo se pueden seguir, documentar y preparar manualmente; sus casillas no se calculan.

## Arquitectura

```
┌──────────────────────────────┐        ┌──────────────────────────────┐
│ Vue 3 + Vite + Pinia         │        │ Extensión Chrome (MV3)       │
│ SPA servida por el backend   │        │ popup + panel en la sede     │
└──────────────┬───────────────┘        └──────────────┬───────────────┘
               │ cookie de sesión HMAC                 │ bearer token revocable
               ▼                                       ▼
        ┌─────────────────────────────────────────────────────┐
        │ Go + chi · API local en 127.0.0.1:8787              │
        │ catálogo YAML · calendario · cálculo · auth · audit │
        └──────────────────────────┬──────────────────────────┘
                                   ▼
                    SQLite + documentos en disco
```

- **Backend** (`backend/`): Go, chi, `modernc.org/sqlite` (sin CGO), migraciones y SPA embebidas.
- **Web** (`web/`): Vue 3, TypeScript, Vite, Pinia, Vue Router, Tailwind CSS 4, `lucide-vue-next`.
- **Extensión** (`extension/`): Manifest V3, TypeScript y esbuild. El popup y la página de opciones usan un diseño propio; el panel se inyecta en un shadow DOM para no interferir con la sede.
- **Catálogo** (`backend/internal/catalog/data/models.yaml`): datos versionados por ejercicio, separados de la lógica de cálculo.
- **Importes**: siempre en céntimos. Los tipos se guardan en puntos básicos (`2100` = 21 %).

## Requisitos

- Go **1.26 o superior**.
- Node.js **20 o superior** (probado con Node 24) y npm solo para compilar la web y la extensión.
- Opcional para OCR local: **Tesseract** y **Poppler** instalados en el sistema. No forman parte del binario; ver [OCR: dependencias del sistema](#ocr-dependencias-del-sistema-opcional).
- Chrome, Edge o Brave basado en Chromium para la extensión. Firefox no está validado.

## Puesta en marcha

```bash
make build-backend  # ERP completo: backend + SPA + migraciones + catálogo
./bin/asesoria      # abre http://127.0.0.1:8787
```

`bin/asesoria` es un único binario Go: incluye la interfaz Vue compilada y no necesita Node ni la carpeta `web/dist` en tiempo de ejecución. `make build` compila además la extensión (`extension/dist`) por separado; la extensión nunca forma parte del binario.

Para probar con datos de demostración:

```bash
./bin/asesoria -demo
```

El primer arranque crea la base de datos y aplica las migraciones. Los datos se guardan por defecto en:

```text
~/.local/share/asesoria/
├── asesoria.db
└── storage/          # documentos content-addressed
```

Puedes cambiarlo con `-data-dir` o `ASESORIA_DATA_DIR`:

```bash
ASESORIA_DATA_DIR=/ruta/a/mis-datos ./bin/asesoria
```

### Biblioteca documental

En **Ajustes → Biblioteca documental** puedes elegir una carpeta del equipo. La aplicación crea y mantiene este árbol para cada contribuyente:

```text
<raíz>/<Nombre (NIF)>/
  2025/
    1T/
      Facturas emitidas/
      Facturas recibidas/
      Facturas/
      Nóminas/
      Justificantes/
      Modelos/
      Recibos/
      Contratos/
      Seguros/
      Banco/
      Otros/
    2T/ … 4T/
    Anual/
    Sin periodo/
  2026/ … 2027/
  Sin clasificar/
```

Se generan los años anterior, actual y siguiente. Al guardar la carpeta, al crear o editar un contribuyente y al escanear, el árbol se vuelve a comprobar; es idempotente y no borra nada. Si un contribuyente se renombra, se reutiliza la carpeta existente que termina en `(NIF)` para no separar sus documentos. Los documentos que no se pueden asignar a ningún contribuyente se archivan en `<raíz>/Sin clasificar/<año>/<periodo>/<tipo>` hasta que se revisan.

La importación es **manual**: pulsa **Escanear biblioteca** en Documentos o Ajustes. El escaneo:

- Recorre la raíz, importa PDF, JPG/PNG/TIFF/WebP/BMP, TXT/MD/CSV, DOCX, XLSX, HTML/XML/JSON y copia cada original dentro del árbol. Los ficheros de origen **no se mueven ni se borran**.
- Deduplica por SHA-256, salta ficheros ya procesados (ruta + fecha + tamaño) y limita cada pasada a 1000 ficheros.
- Clasifica por la carpeta, el NIF y el contenido; lo que no puede confirmar queda en la cola **por revisar** de Documentos. Nunca marca un modelo como presentado ni crea facturas automáticamente.
- Extrae texto de PDF con capa de texto, TXT/CSV/MD, DOCX y XLSX. Para PDF escaneados e imágenes intenta OCR local con Tesseract.

No se usa OCR en la nube: todo ocurre en el equipo. La extracción no está implementada como librería Go; el binario invoca herramientas locales mediante `exec.LookPath` y `exec.Command`. No hay dependencias Go adicionales ni CGO por esta función.

#### OCR: dependencias del sistema (opcional)

| Ejecutable | Paquete / origen | Uso |
| --- | --- | --- |
| `pdftotext` | Poppler | Extrae la capa de texto de PDF digitales. |
| `pdftoppm` | Poppler | Rasteriza PDF escaneados para pasarlos a Tesseract. |
| `tesseract` | Tesseract OCR + datos de idioma (`spa`, `eng`) | OCR de imágenes y de páginas rasterizadas. |

Instalación orientativa:

```bash
# Debian/Ubuntu
sudo apt install poppler-utils tesseract-ocr tesseract-ocr-spa

# macOS (Homebrew)
brew install poppler tesseract tesseract-lang
```

En **Windows** no hay un paquete oficial único: instala [Tesseract para Windows (UB Mannheim)](https://github.com/UB-Mannheim/tesseract/wiki) con el idioma español y un build de [Poppler para Windows](https://github.com/oschwartz10612/poppler-windows/releases). Añade al `PATH` las carpetas que contienen `tesseract.exe`, `pdftotext.exe` y `pdftoppm.exe`. En PowerShell puedes comprobar la detección con `where.exe tesseract`, `where.exe pdftotext` y `where.exe pdftoppm`.

Comprobación en Linux/macOS:

```bash
tesseract --version
tesseract --list-langs   # debe incluir spa
pdftotext -v
pdftoppm -v
```

La aplicación detecta estas herramientas en tiempo de ejecución y muestra su estado en **Ajustes → Biblioteca documental** (`tesseract`, `pdf_text`, `pdf_images`). Sin Tesseract los documentos se archivan igualmente y se marcan como `OCR no disponible`; puedes instalarlo más tarde y usar **Reprocesar texto** desde la revisión del documento. Sin Poppler no se procesan PDF; con Poppler y sin Tesseract solo se leen PDF con capa de texto. El OCR procesa como máximo las 10 primeras páginas de un PDF a 200 dpi y el idioma se configura en Ajustes (`spa`, `eng` o `spa+eng`).

El binario Go se compila para Linux, macOS y Windows sin CGO. Las herramientas de OCR no se empaquetan dentro del binario y deben instalarse por separado en cada equipo. El código usa los nombres estándar (`tesseract`, `pdftotext`, `pdftoppm`) y no contiene rutas específicas de plataforma; la compilación cruzada está verificada, aunque conviene comprobar la detección en cada equipo después de instalarlas.

La biblioteca es opcional: si dejas la carpeta vacía, las subidas siguen usando el almacén content-addressed de `data-dir` y no se crea ningún árbol. Cambiar de raíz no mueve los documentos antiguos: cada documento recuerda su raíz original para poder descargarlo o borrarlo.

### Desarrollo

En dos terminales:

```bash
make dev-backend    # API en 127.0.0.1:8787, permite el origen de Vite
make dev-web        # SPA en http://localhost:5173 con proxy /api
```

El binario sirve primero una carpeta externa si se indica con `-web-dir` (útil para desarrollo o para probar un build concreto). Si no existe `index.html`, usa la SPA embebida. Para generar el binario autónomo:

```bash
make build-backend
./bin/asesoria
```

`make embed-web` copia el resultado de Vite a `backend/internal/webapp/dist`, que es lo que embebe Go. Esa carpeta está ignorada por Git salvo `.keep`, que permite compilar antes de construir la web.

### Variables y flags

| Flag | Variable | Por defecto |
| --- | --- | --- |
| `-addr` | `ASESORIA_ADDR` | `127.0.0.1:8787` |
| `-data-dir` | `ASESORIA_DATA_DIR` | `~/.local/share/asesoria` |
| `-web-dir` | `ASESORIA_WEB_DIR` | vacío (usa la SPA embebida; valor opcional para desarrollo) |
| `-dev` | `ASESORIA_DEV=1` | desactivado |
| `-demo` | `ASESORIA_DEMO=1` | desactivado |

No expongas el servidor en una interfaz pública. El modelo de seguridad asume loopback y un único usuario.

## Extensión

```bash
cd extension
npm ci
npm run build       # genera extension/dist
```

1. Abre `chrome://extensions`.
2. Activa el **modo de desarrollador**.
3. Pulsa **Cargar descomprimida** y selecciona `extension/dist`. El build también deja los ficheros compilados en `extension/`, así que cargar esa carpeta tras `npm run build` funciona igual. Usa siempre la misma ruta: Chrome identifica la extensión por su carpeta y, si cambias de una a otra, tendrás que emparejarla de nuevo.
4. En el ERP entra en **Ajustes → Conectar la extensión** y genera un código.
5. Abre el popup de la extensión, pega el código y pulsa **Conectar**.

El código caduca en 10 minutos y solo puede usarse una vez. El token resultante se guarda en `chrome.storage.local`, se guarda hasheado en el servidor y puede revocarse desde **Ajustes → Accesos de extensiones**. Los tokens de extensión son **solo de lectura**: no pueden crear, modificar ni borrar datos del ERP.

### Qué hace el popup

- Muestra los **próximos 8 vencimientos** desde hoy, agrupados por mes, con la fecha exacta y una cuenta atrás («Vence hoy», «Vence en X días»).
- Permite cambiar a **Todos** para consultar el resto de expedientes abiertos, incluidos los vencidos, y buscar por modelo, periodo o NIF.
- Al abrir un expediente, lo marca como activo para el panel de la sede.

### Qué hace el panel de la sede

- Detecta el expediente abierto desde el popup y muestra modelo, periodo, vencimiento, estado, casillas calculadas, avisos y documentos.
- Permite copiar una casilla, copiar todas o descargar documentos.
- Abre el ERP o la sede oficial.
- Se cierra con la X y limpia el expediente activo.

### Adaptadores de formulario

`extension/src/adapters/index.ts` define un motor declarativo (`wait`, `set`, `select`, `click`, `pause`) y **no incluye ningún adaptador verificado**. El panel funciona siempre como asistente manual. Para habilitar relleno automático:

1. Verifica los selectores contra el formulario oficial del ejercicio concreto.
2. Añade el adaptador con `verified: true` solo después de probarlo.
3. Termina siempre con un paso `pause` para que el usuario revise y firme.

La extensión no rellena, envía ni firma sin intervención del usuario, y nunca solicita ni lee el certificado.

## Desarrollo, issues y releases

El trabajo se organiza con **issues** y **pull requests** contra `main`. La guía completa está en [CONTRIBUTING.md](CONTRIBUTING.md).

- Los títulos de las PRs siguen Conventional Commits (`feat:`, `fix:`, `docs:`, `security:`, etc.). El workflow de etiquetado aplica la etiqueta de release correspondiente.
- Las releases se publican creando un tag `vX.Y.Z` sobre `main`:

```bash
git tag -a v0.2.0 -m "Asesoría v0.2.0"
git push origin v0.2.0
```

El workflow de release compila la web y la incrusta en el binario, genera la extensión, compila binarios para Linux, macOS y Windows, publica los checksums y crea la release. Las notas se generan agrupadas por categorías (`.github/release.yml`): novedades, correcciones, seguridad, documentación, mantenimiento y otros cambios. Las PRs sin etiqueta aparecen en **Otros cambios**; añade `skip-changelog` para excluirlas.

Tras descargar un binario de Linux o macOS, dale permisos de ejecución (`chmod +x asesoria_...`) antes de arrancarlo. La extensión se distribuye como un ZIP; descomprímelo y carga la carpeta resultante desde `chrome://extensions`.

## Catálogo y calendario

El catálogo está en `backend/internal/catalog/data/models.yaml`. Declara `version` y `updated_at`, la fecha de la última revisión de sus fichas. Cada modelo puede sobrescribir esa fecha con su propio `updated_at`. La aplicación muestra esa fecha como **Actualizado**; se mantiene al revisar el modelo contra la sede oficial y no se obtiene automáticamente.

Cada modelo declara:

- `code`, `name`, `entity`, `category`, `periodicity`, `description` (explicación de para qué sirve), `url`.
- `updated_at` (opcional): fecha de la última actualización de esa ficha. Si se omite, hereda la del catálogo.
- `obsolete` (opcional): conserva un modelo histórico sin tratarlo como vigente. El 037 está marcado así desde su supresión en 2025.
- `applies`: forma jurídica, régimen de IVA, método de IRPF, empleados, alquileres o actividad económica.
- `schedule`: `monthly`, `quarterly`, `annual` o `custom`, con meses, día de vencimiento y offsets de periodo.
- `documents`: checklist documental orientativa con los soportes habituales para cumplimentar el modelo; la ficha oficial puede exigir únicamente el modelo.
- `auto`: si se genera automáticamente en el calendario.

Los vencimientos se generan por **año de vencimiento**. Por eso el 4T de 2026 aparece en el calendario de 2027. Los sábados y domingos se trasladan al lunes.

### Cálculo

Solo 303 y 130 tienen calculadora:

- **303**: casillas 01–28 (bases, tipos, cuotas devengadas/deducibles y resultado del régimen general).
- **130**: casillas 01–07 (ingresos, gastos, rendimiento neto, 20 %, retenciones y pagos previos).

Los cálculos usan las facturas del periodo y contribuyente. **Las correspondencias de casillas están pendientes de validar contra el formulario oficial de 2026**; la interfaz muestra este aviso. Los regímenes especiales de IVA no están cubiertos.

## Seguridad y privacidad

- El servidor solo acepta `Host` de loopback y orígenes de extensión/localhost.
- La SPA se autentica con una cookie `HttpOnly`, `SameSite=Strict`, firmada con HMAC.
- El emparejamiento usa códigos de 6 dígitos de un solo uso, con caducidad y almacenamiento transaccional.
- Los tokens de extensión se guardan hasheados (SHA-256), son revocables y solo permiten `GET`.
- Los documentos se guardan en disco y se referencian por SHA-256; no se sirven desde rutas arbitrarias.
- La biblioteca solo escribe dentro de la carpeta configurada por el usuario. El OCR se ejecuta con herramientas locales y no envía contenido a terceros.
- No hay acceso al certificado ni a la clave privada: la firma ocurre siempre en el navegador.
- No hay telemetría, analítica ni llamadas a servicios de terceros.

**Límites del modelo:** es una aplicación local de un solo usuario. No hay cifrado en reposo de SQLite ni de los documentos; protege la cuenta del sistema operativo y la carpeta de datos. Cualquier proceso local con acceso a tu usuario podría leer los datos. Para copias de seguridad, detén la aplicación y copia el directorio de datos completo.

## Aviso legal

Esta herramienta ayuda a organizar, calcular y documentar, pero **no sustituye el asesoramiento profesional** ni garantiza la corrección de una autoliquidación. Verifica siempre las casillas, los plazos y la normativa aplicable antes de firmar. La presentación y la firma son responsabilidad del usuario y se realizan en la sede oficial con su certificado.

Si en el futuro la aplicación emite facturas por cuenta del contribuyente, deberá cumplir Verifactu y la normativa de facturación electrónica aplicable. En esta versión solo se registran e importan facturas ya emitidas.

## Estructura

```text
backend/
  cmd/asesoria/        arranque y flags
  internal/api/        rutas HTTP, auth, CORS, SPA
  internal/calendar/   generación de vencimientos
  internal/calc/       calculadoras 303 y 130
  internal/catalog/    carga del YAML embebido
  internal/db/         SQLite y migraciones
  internal/library/    árbol, escaneo, clasificación y OCR local
  internal/seed/       datos de demostración
  internal/storage/    documentos en disco
  internal/store/      acceso a datos y tokens
  internal/webapp/     SPA Vue embebida con //go:embed
web/
  src/pages/           dashboard, calendario, expediente, facturas…
  src/components/      UI reutilizable
  src/api.ts           cliente tipado y helpers de importes/fechas
extension/
  src/popup.ts         emparejamiento y listado
  src/options.ts       ajustes y estado de conexión
  src/content.ts       panel sobre la sede
  src/adapters/        motor declarativo (sin adaptadores verificados)
  src/background.ts    router de mensajes y descargas
.github/
  workflows/           CI, release y automatización de etiquetas
  ISSUE_TEMPLATE/      plantillas de issues
CONTRIBUTING.md        flujo de issues, PRs y releases
Makefile
```

## Comprobaciones

```bash
make test     # Go tests + build web + typecheck extensión
make check    # gofmt, go vet, vue-tsc y tsc
```

## Problemas frecuentes

- **El puerto 8787 está ocupado**: usa `./bin/asesoria -addr 127.0.0.1:8790` y cambia la dirección en la extensión.
- **“La interfaz web no está embebida en este binario”**: compila el ERP con `make build-backend`; el binario incorpora la SPA y no depende de `web/dist` en ejecución.
- **La extensión no conecta**: comprueba que el backend está en marcha, que la dirección coincide y que el código no ha caducado. Genera uno nuevo si es necesario.
- **El panel no aparece en la sede**: abre el expediente desde el popup de la extensión. El panel solo se monta cuando hay un expediente activo y la extensión está emparejada.
- **El escaneo no lee PDF escaneados o imágenes**: instala Poppler y Tesseract (ver [Dependencias de OCR](#ocr-dependencias-del-sistema-opcional)), comprueba que `tesseract`, `pdftotext` y `pdftoppm` están en el `PATH` (`where.exe` en Windows, `command -v` en Linux/macOS), reinicia la aplicación, vuelve a Ajustes y pulsa **Escanear ahora**, o usa **Reprocesar texto** en la revisión del documento.
- **Un documento aparece “por revisar”**: el clasificador no pudo confirmar el contribuyente, el tipo o la fecha. Abre el documento, corrige los campos y guarda; saldrá de la cola. Los documentos no cambian de carpeta al editar sus metadatos.
- **Fechas o importes raros**: revisa la zona horaria del equipo y que los importes se introduzcan en formato español (`1.234,56`) o con punto decimal (`1234.56`).
