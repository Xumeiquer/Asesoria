# Contribuir a Asesoría

Gracias por el interés. El proyecto se organiza con **issues** y **pull requests**; los cambios directos a `main` se reservan para tareas de mantenimiento excepcionales.

## Antes de empezar

- Busca una issue existente antes de abrir una nueva. Si el problema es distinto, abre una issue con la plantilla correspondiente.
- Usa datos ficticios o el modo `-demo`. No adjuntes bases de datos reales, certificados, contraseñas ni datos personales.
- Para cambios grandes, abre primero una issue o una propuesta para acordar el alcance.

## Issues

Hay tres plantillas:

- **Informe de error** (`bug`): fallos reproducibles, con pasos, resultado esperado y entorno.
- **Propuesta de mejora** (`enhancement`): funcionalidad nueva o cambio de comportamiento.
- **Formulario oficial** (`enhancement`): alta o actualización de un adaptador en `forms/`.

Las issues se etiquetan por tipo; la plantilla recoge además el área afectada. La etiqueta de tipo es la que determina la categoría del changelog de la release.

## Formularios oficiales

Los adaptadores de relleno viven en `forms/` y se declaran en YAML o JSON. No hace falta tocar TypeScript para añadir un modelo, un ejercicio o una revisión nueva. La guía completa está en [`forms/README.md`](forms/README.md).

Reglas rápidas:

- Sigue la ruta `forms/<organismo>/<modelo>/<ejercicio>/<version>.yaml`. El build valida que coincida con los metadatos.
- Un cambio del formulario se representa con una revisión nueva (`2026.2.yaml`), no sobrescribiendo una versión ya verificada.
- Empieza con `verified: false` y cambia a `true` solo después de probar los selectores contra el formulario oficial o su simulador.
- Usa selectores estables por `aria-label`, etiqueta o estructura; nunca IDs dinámicos de ZK.
- No incluyas datos personales, NIF reales, importes reales, certificados ni secretos. Usa el simulador o HTML saneado.
- La extensión solo rellena y revisa: no firma, no presenta y no accede al certificado, Cl@ve ni DNIe.
- Ejecuta `npm run forms:check` y `make check` antes de abrir la PR.

El catálogo también alimenta la ficha del ERP, que muestra qué casillas y datos necesita el modelo antes de presentarlo.

## Ramas y commits

Crea una rama corta a partir de `main`:

```bash
git checkout main
git pull
git checkout -b feat/calendario-por-trimestre
```

Los commits siguen [Conventional Commits](https://www.conventionalcommits.org/es/v1.0.0/):

```text
feat(web): añade filtro por trimestre en el calendario
fix(extension): evita perder el expediente al abrir la sede
docs: documenta el flujo de releases
chore(ci): actualiza la versión de Node
```

Prefijos admitidos: `feat`, `fix`, `docs`, `chore`, `refactor`, `ci`, `deps` y `security`. El scope es opcional (`web`, `backend`, `extension`, `catalog`, `ci`).

## Pull requests

1. Abre la PR contra `main`.
2. Usa un título con el mismo formato que los commits: `tipo(scope): descripción`.
3. Rellena la plantilla, enlaza la issue con `Closes #123` y explica cómo probar el cambio.
4. Espera a que pase **CI** (Go, web, extensión y formularios).
5. Mantén la PR pequeña y con un objetivo claro. Si mezcla cambios independientes, sepáralos.

El workflow de etiquetado aplica automáticamente la etiqueta de release a partir del prefijo del título. También puedes corregirla a mano si hace falta.

| Prefijo | Etiqueta | Categoría en la release |
| --- | --- | --- |
| `feat` | `enhancement` | 🚀 Novedades |
| `fix` | `bug` | 🐛 Correcciones |
| `security` | `security` | 🔒 Seguridad |
| `docs` | `documentation` | 📚 Documentación |
| `chore`, `refactor`, `ci`, `deps` | `chore`, `refactor`, `ci`, `dependencies` | 🧰 Mantenimiento |

Añade `skip-changelog` si la PR no debe aparecer en las notas de la release.

## Configuración recomendada del repositorio

Para que el flujo se cumpla, protege `main` en **Settings → Branches**:

- Requerir una pull request antes de fusionar.
- Requerir que pasen los checks de **CI**.
- Bloquear force pushes y borrados de la rama.
- Activar el borrado automático de ramas al fusionar.

Las etiquetas se crean o actualizan con `.github/workflows/labels.yml` a partir de `.github/labels.json`; también puedes lanzarlo a mano desde **Actions**.

## Comprobaciones locales

Antes de pedir revisión:

```bash
make check
```

Esto valida el catálogo de formularios, comprueba formato y vet de Go, el typecheck de la web y el de la extensión, y ejecuta las pruebas de la extensión. Para probar el build completo:

```bash
make build
```

Si solo has tocado formularios, la comprobación rápida es:

```bash
npm ci
npm run forms:check
```

Para trabajar solo con la extensión, instala primero las dependencias del generador en la raíz:

```bash
npm ci
cd extension
npm ci
npm run build
```

Después, en `chrome://extensions`, recarga la extensión desde `extension/dist` (o desde `extension/` si la cargaste así).

## Releases

Las releases se publican creando un tag de versión sobre `main`. Por ahora son estables (`vX.Y.Z`), porque el `manifest.json` de la extensión debe usar una versión numérica:

```bash
git checkout main
git pull
git tag -a v0.2.0 -m "Asesoría v0.2.0"
git push origin v0.2.0
```

El workflow `.github/workflows/release.yml`:

1. Compila la web y la incrusta en el binario.
2. Compila la extensión y ajusta su `manifest.version` a la versión del tag.
3. Genera binarios para Linux, macOS y Windows en `amd64` y `arm64`.
4. Calcula `SHA256SUMS`.
5. Crea la release con notas generadas automáticamente y agrupadas por las categorías de `.github/release.yml`.

Para que el changelog salga bien, las PRs deben tener la etiqueta correcta antes de fusionarse. Las PRs sin etiqueta aparecen en **Otros cambios**. Las etiquetas `skip-changelog`, `duplicate`, `invalid` y `wontfix` se excluyen.

La versión del binario se inyecta desde el tag (`-X main.version=...`); no hace falta editar `backend/cmd/asesoria/main.go` para cada release. La extensión también recibe la versión del tag durante el build de la release.

Si el workflow falla después de crear la release, elimina esa release (o el tag, si quieres volver a empezar) antes de relanzarlo: `gh release create` no sobrescribe una release existente.
