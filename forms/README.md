# Catálogo declarativo de formularios

Esta carpeta contiene la definición de cómo rellenar los formularios oficiales. Es el **único sitio** donde se declaran los selectores, las casillas y los datos que necesita el ERP; la extensión y la web se generan a partir de estos ficheros en tiempo de compilación.

Se admiten **YAML**, **YML** y **JSON**. YAML es el formato recomendado para las aportaciones porque es más legible y permite comentarios.

## Principios

- **Solo rellena y revisa.** Ningún adaptador puede firmar, presentar, acceder al certificado, a Cl@ve, al DNIe ni a la clave de firma.
- **Previsualización obligatoria.** La extensión muestra primero los cambios, el valor actual y el nuevo, y solo escribe después de una confirmación explícita.
- **Selectores estables.** Usa `aria-label`, etiquetas o estructura del formulario. No uses IDs dinámicos de ZK ni rutas que cambien entre sesiones.
- **Sin datos reales.** Las contribuciones no deben incluir NIF, nombres, importes ni HTML con datos personales. Usa el simulador o HTML saneado.
- **Versionado.** Un cambio de la Administración se representa con una revisión nueva; no se sobrescribe silenciosamente una versión validada.

## Estructura

La ruta forma parte del contrato y se valida en el build:

```text
forms/
  README.md
  schema/adapter.schema.json
  <organismo>/<modelo>/<ejercicio>/<version>.yaml
```

Ejemplos:

```text
forms/aeat/303/2026/2026.1.yaml
forms/aeat/303/2026/2026.2.yaml
forms/aeat/130/2026/2026.1.yaml
forms/tgss/.../.../....yaml
forms/registro-mercantil/.../.../....yaml
```

El nombre del fichero debe coincidir con `version`, y la carpeta con `entity`, `model` y `exercise`. El generador comprueba esta convención y rechaza el fichero si no coincide.

## Formato

Ejemplo mínimo en YAML:

```yaml
schemaVersion: 1
entity: AEAT
model: "303"
exercise: "2026"
version: "2026.1"
updatedAt: "2026-10-30"
verified: true
verifiedAgainst: "https://www2.agenciatributaria.gob.es/wlpl/A303-FWME/E2026/OPEN/index.zul"
page:
  contains:
    - "IVA devengado"
    - "Total cuota devengada"
pageHint: "Abre la página «Liquidación: régimen general» y vuelve a intentarlo."
notes: "Selectores probados contra el simulador oficial. Revisa los importes antes de firmar."
fields:
  - casilla: "01"
    label: "Base imponible al 4%"
    selector: 'input[aria-label$="01"]'
    kind: amount
  - casilla: "03"
    label: "Cuota devengada al 4%"
    selector: 'input[aria-label$="03"]'
    kind: amount
```

El mismo contenido en JSON:

```json
{
  "schemaVersion": 1,
  "entity": "AEAT",
  "model": "303",
  "exercise": "2026",
  "version": "2026.1",
  "updatedAt": "2026-10-30",
  "verified": false,
  "page": { "contains": ["IVA devengado"] },
  "fields": [
    {
      "casilla": "01",
      "label": "Base imponible al 4%",
      "selector": "input[aria-label$=\"01\"]",
      "kind": "amount"
    }
  ]
}
```

## Campos del adaptador

| Campo | Obligatorio | Descripción |
| --- | --- | --- |
| `schemaVersion` | Sí | Versión del formato. Actualmente `1`. |
| `entity` | Sí | `AEAT`, `TGSS` o `Registro Mercantil`. |
| `model` | Sí | Código del modelo o trámite, como `"303"`. |
| `exercise` | Sí | Ejercicio de cuatro dígitos, como `"2026"`. |
| `version` | Sí | Revisión dentro del ejercicio, como `"2026.1"` o `"2026.2.1"`. |
| `updatedAt` | Sí | Fecha ISO de la revisión, `AAAA-MM-DD`. |
| `verified` | Sí | `true` solo después de probar los selectores. |
| `verifiedAgainst` | Si `verified` es `true` | URL del formulario o simulador usado. |
| `page.contains` | Sí | Textos que deben aparecer para considerar que la página es la correcta. |
| `pageHint` | No | Instrucción que se muestra si el usuario está en otra página. |
| `notes` | No | Notas para revisores o usuarios. |
| `fields` | Sí | Lista de campos editables. |

### Campos de `fields`

| Campo | Obligatorio | Descripción |
| --- | --- | --- |
| `casilla` | Uno de `casilla` o `source` | Identificador de la casilla, como `"01"`. |
| `label` | Sí | Nombre legible; se usa como respaldo si el cálculo no trae etiqueta. |
| `selector` | Sí | Selector CSS estable del campo. |
| `kind` | No | `text`, `amount`, `percent`, `date`, `nif`, `select` o `checkbox`. Por defecto `text`. |
| `source` | Uno de `casilla` o `source` | Expresión con placeholders. Si se omite y hay `casilla`, se usa `{{casilla.<casilla>}}`. |
| `optional` | No | El campo puede no existir en la página según el perfil fiscal. No impide el relleno. |
| `required` | No | El ERP debe tener un valor antes de rellenar. Por defecto `true`. |

`optional` y `required` no significan lo mismo:

- `optional: true`: el campo puede no estar presente en esa variante del formulario.
- `required: false`: el dato no se exige en la comprobación previa del ERP ni bloquea el relleno si falta.

Para una operación que solo aplica a algunos contribuyentes suele tener sentido `optional: true` y `required: false`. Si el campo siempre existe y el cálculo debe aportarlo, deja `required` sin poner.

## Placeholders

El motor admite las mismas expresiones que el resto de la aplicación:

```text
{{casilla.01}}
{{taxpayer.nif}}
{{taxpayer.name}}
{{taxpayer.legal_form}}
{{filing.id}}
{{filing.model_code}}
{{filing.model_name}}
{{filing.entity}}
{{filing.period}}
{{filing.period_label}}
{{filing.period_start}}
{{filing.period_end}}
{{filing.due_date}}
{{exercise}}
```

Los importes se formatean con coma decimal y los tipos con el formato oficial. No escribas formateos manuales en `source`; usa el placeholder y deja que el motor aplique el formato según `kind`.

## Versionado y cambios de la Administración

1. Si el formulario cambia, **crea un fichero nuevo** con la siguiente revisión: `2026.2.yaml`.
2. Conserva la revisión anterior si el formulario antiguo sigue accesible.
3. Empieza la revisión nueva con `verified: false` y `updatedAt` actualizado.
4. Actualiza `page.contains`, `pageHint`, selectores y campos.
5. Valida contra el formulario o simulador y cambia `verified` a `true` solo entonces.
6. Rellena `verifiedAgainst` con la URL exacta usada.

La extensión ordena las revisiones de más reciente a más antigua. Para un expediente, intenta primero la revisión más nueva cuya página coincide; si no coincide, puede usar una anterior verificada y, si ninguna coincide, muestra la pista de la más reciente.

## Build y validación

Desde la raíz del repositorio:

```bash
npm ci                 # instala yaml y ajv para el generador
npm run forms:check    # valida sin escribir artefactos
npm run forms          # valida y genera extensión + web
```

`make forms`, `make check` y `make build` ejecutan la generación automáticamente. Los artefactos generados están ignorados por Git:

```text
extension/src/adapters/generated.ts
web/src/generated/forms.json
```

No edites esos ficheros a mano. Si cambias un YAML o JSON, vuelve a ejecutar `npm run forms` o cualquiera de los targets de Make.

La validación comprueba:

- que el fichero cumple `forms/schema/adapter.schema.json`;
- que la ruta coincide con organismo, modelo, ejercicio y versión;
- que no hay adaptadores duplicados para la misma revisión;
- que no hay campos duplicados;
- que `verified: true` incluye `verifiedAgainst`;
- que los placeholders son conocidos y tienen forma válida.

## Cómo verificar un adaptador

1. Abre el formulario oficial o su simulador público. Si necesitas el formulario autenticado, usa HTML saneado y no adjuntes datos reales.
2. Comprueba que `page.contains` identifica la página correcta y que `pageHint` explica cómo llegar.
3. Inspecciona cada campo y confirma que el selector es estable y único en esa página.
4. Prepara un expediente de demostración con valores conocidos.
5. Abre la extensión, pulsa **Revisar relleno** y compara la previsualización con el formulario.
6. Aplica el relleno y revisa que los importes y formatos son correctos.
7. No firmes ni presentes desde la extensión: la firma se hace siempre manualmente en el navegador.
8. Documenta la URL y la fecha en `verifiedAgainst` y `updatedAt`.

## Qué ocurre en el ERP

El generador crea un catálogo público sin selectores con los requisitos de cada adaptador. La ficha del expediente muestra:

- el estado del adaptador (verificado o en preparación);
- las casillas y datos que necesita el formulario;
- cuáles están presentes en el cálculo y cuáles faltan;
- un acceso directo a calcular si todavía no hay valores.

La extensión usa la misma información para bloquear el relleno cuando falta un valor marcado como requerido. El objetivo es que el usuario complete el ERP antes de abrir el formulario oficial, no que descubra el hueco a mitad del relleno.

## Seguridad y privacidad

- Nunca incluyas certificados, contraseñas, tokens, NIF reales, nombres ni importes reales.
- No añadas selectores que pulsen botones de firma, presentación, borrado o envío.
- No automatices la navegación si puede tener efectos irreversibles. Este catálogo está pensado para rellenar la página actual y dejar la revisión y la firma en manos del usuario.
- Si encuentras un comportamiento sospechoso en el HTML de la sede, descríbelo en la issue sin pegar datos personales.
