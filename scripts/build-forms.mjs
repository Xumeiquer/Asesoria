#!/usr/bin/env node
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { parseArgs } from 'node:util'

import Ajv from 'ajv'
import { parse as parseYaml } from 'yaml'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const formsDir = path.join(root, 'forms')
const schemaFile = path.join(formsDir, 'schema', 'adapter.schema.json')
const extensionFile = path.join(root, 'extension', 'src', 'adapters', 'generated.ts')
const webFile = path.join(root, 'web', 'src', 'generated', 'forms.json')

const FORMATS = new Set(['.yaml', '.yml', '.json'])
const PLACEHOLDER = /\{\{\s*([A-Za-z0-9_.]+)\s*\}\}/g
const CONTEXT_KEYS = new Set([
  'taxpayer.nif',
  'taxpayer.name',
  'taxpayer.legal_form',
  'filing.id',
  'filing.model_code',
  'filing.model_name',
  'filing.entity',
  'filing.period',
  'filing.period_label',
  'filing.period_start',
  'filing.period_end',
  'filing.due_date',
  'exercise',
])

function slug(value) {
  return value
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function relative(file) {
  return path.relative(root, file)
}

async function walk(dir) {
  const files = []
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    if (entry.name.startsWith('.')) continue
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'schema') continue
      files.push(...(await walk(full)))
    } else if (FORMATS.has(path.extname(entry.name))) {
      files.push(full)
    }
  }
  return files
}

function parseFile(text, file) {
  const ext = path.extname(file)
  try {
    return ext === '.json' ? JSON.parse(text) : parseYaml(text)
  } catch (error) {
    throw new Error(`${relative(file)}: no se puede parsear (${error.message})`)
  }
}

function validatePath(adapter, file) {
  const ext = path.extname(file)
  const segments = path.relative(formsDir, file).split(path.sep)
  const expected = [slug(adapter.entity), adapter.model, adapter.exercise, `${adapter.version}${ext}`]
  if (
    segments.length !== expected.length ||
    segments.some((segment, index) => segment !== expected[index])
  ) {
    throw new Error(
      `${relative(file)}: la ruta debe ser forms/${expected.join('/')} ` +
        `(entity/model/exercise/version)`,
    )
  }
}

function validateFields(adapter, file) {
  const labels = new Set()
  for (const [index, field] of adapter.fields.entries()) {
    const key = field.casilla ?? field.label
    if (labels.has(key)) {
      throw new Error(`${relative(file)}: campo duplicado «${key}» (posición ${index + 1})`)
    }
    labels.add(key)

    if (field.source) {
      const matches = [...field.source.matchAll(PLACEHOLDER)]
      const leftover = field.source.replace(PLACEHOLDER, '')
      if (
        leftover.includes('{{') ||
        leftover.includes('}}') ||
        (field.source.includes('{{') && !matches.length)
      ) {
        throw new Error(`${relative(file)}: placeholder inválido en «${field.label}»: ${field.source}`)
      }
      for (const [, placeholderKey] of matches) {
        if (placeholderKey.startsWith('casilla.')) {
          if (!placeholderKey.slice('casilla.'.length)) {
            throw new Error(`${relative(file)}: casilla vacía en «${field.label}»: ${field.source}`)
          }
          continue
        }
        if (!CONTEXT_KEYS.has(placeholderKey)) {
          throw new Error(
            `${relative(file)}: placeholder desconocido en «${field.label}»: {{${placeholderKey}}}`,
          )
        }
      }
    }
  }
}

function normalizeField(field) {
  const source = field.source ?? (field.casilla ? `{{casilla.${field.casilla}}}` : undefined)
  return {
    ...(field.casilla ? { casilla: field.casilla } : {}),
    label: field.label.trim(),
    selector: field.selector.trim(),
    kind: field.kind ?? 'text',
    ...(source ? { source } : {}),
    ...(field.optional ? { optional: true } : {}),
    ...(field.required === false ? { required: false } : {}),
  }
}

function placeholders(source) {
  return [...source.matchAll(PLACEHOLDER)].map((match) => match[1])
}

function requirements(fields) {
  const seen = new Set()
  const result = []
  for (const field of fields) {
    if (field.required === false) continue
    for (const key of placeholders(field.source ?? '')) {
      if (seen.has(key)) continue
      seen.add(key)
      result.push({
        key,
        ...(key.startsWith('casilla.') ? { casilla: key.slice('casilla.'.length) } : {}),
        label: field.label,
        kind: field.kind,
        required: true,
      })
    }
  }
  return result
}

function compareVersion(a, b) {
  const left = a.split('.').map((part) => Number(part))
  const right = b.split('.').map((part) => Number(part))
  for (let index = 0; index < Math.max(left.length, right.length); index++) {
    const difference = (right[index] ?? 0) - (left[index] ?? 0)
    if (difference !== 0) return difference
  }
  return 0
}

function sortAdapters(adapters) {
  return adapters.sort(
    (left, right) =>
      left.entity.localeCompare(right.entity) ||
      left.code.localeCompare(right.code, undefined, { numeric: true }) ||
      left.exercise.localeCompare(right.exercise) ||
      compareVersion(left.version, right.version),
  )
}

function formatValidationError(file, errors) {
  const details = (errors ?? [])
    .map((error) => `  - ${error.instancePath || '/'} ${error.message ?? 'valor no válido'}`)
    .join('\n')
  return `${relative(file)}: no cumple el esquema\n${details}`
}

async function loadAdapters() {
  const schema = JSON.parse(await readFile(schemaFile, 'utf8'))
  const ajv = new Ajv({ allErrors: true, strict: false })
  const validate = ajv.compile(schema)
  const files = (await walk(formsDir)).sort()
  const adapters = []
  const seen = new Set()

  for (const file of files) {
    const raw = parseFile(await readFile(file, 'utf8'), file)
    if (!validate(raw)) throw new Error(formatValidationError(file, validate.errors))
    validatePath(raw, file)
    validateFields(raw, file)

    const normalized = {
      entity: raw.entity,
      code: raw.model,
      exercise: raw.exercise,
      version: raw.version,
      updatedAt: raw.updatedAt,
      verified: raw.verified,
      ...(raw.verifiedAgainst ? { verifiedAgainst: raw.verifiedAgainst } : {}),
      page: raw.page,
      ...(raw.pageHint ? { pageHint: raw.pageHint } : {}),
      ...(raw.notes ? { notes: raw.notes } : {}),
      fields: raw.fields.map(normalizeField),
    }

    const key = `${normalized.entity}/${normalized.code}/${normalized.exercise}/${normalized.version}`
    if (seen.has(key)) throw new Error(`${relative(file)}: adaptador duplicado ${key}`)
    seen.add(key)
    normalized.requirements = requirements(normalized.fields)
    adapters.push(normalized)
  }

  if (!adapters.length) throw new Error('No hay formularios en forms/')
  return sortAdapters(adapters)
}

function extensionSource(adapters) {
  return `// Generated by scripts/build-forms.mjs. Do not edit.\nimport type { Adapter } from './types'\n\nexport const generatedAdapters = ${JSON.stringify(
    adapters.map(({ requirements: _requirements, ...adapter }) => adapter),
    null,
    2,
  )} as Adapter[]\n`
}

function webSource(adapters) {
  return `${JSON.stringify(
    {
      generatedAt: new Date().toISOString(),
      adapters: adapters.map((adapter) => ({
        entity: adapter.entity,
        model: adapter.code,
        exercise: adapter.exercise,
        version: adapter.version,
        updatedAt: adapter.updatedAt,
        verified: adapter.verified,
        ...(adapter.verifiedAgainst ? { verifiedAgainst: adapter.verifiedAgainst } : {}),
        ...(adapter.pageHint ? { pageHint: adapter.pageHint } : {}),
        ...(adapter.notes ? { notes: adapter.notes } : {}),
        page: adapter.page,
        requirements: adapter.requirements,
      })),
    },
    null,
    2,
  )}\n`
}

async function main() {
  const { values } = parseArgs({
    options: {
      check: { type: 'boolean', default: false },
      target: { type: 'string', default: 'all' },
      help: { type: 'boolean', short: 'h', default: false },
    },
  })

  if (values.help) {
    console.log('Uso: node scripts/build-forms.mjs [--check] [--target all|web|extension]')
    return
  }
  if (!['all', 'web', 'extension'].includes(values.target)) {
    throw new Error('--target debe ser all, web o extension')
  }

  const adapters = await loadAdapters()
  const label = values.check ? 'validados' : 'generados'
  console.log(`Formularios ${label}: ${adapters.length}`)
  for (const adapter of adapters) {
    console.log(
      `  - ${adapter.entity} ${adapter.code}/${adapter.exercise} v${adapter.version} ` +
        `(${adapter.verified ? 'verificado' : 'pendiente'}, ${adapter.fields.length} campos)`,
    )
  }

  if (values.check) return

  if (values.target === 'all' || values.target === 'extension') {
    if (!existsSync(path.dirname(extensionFile))) {
      await mkdir(path.dirname(extensionFile), { recursive: true })
    }
    await writeFile(extensionFile, extensionSource(adapters))
  }
  if (values.target === 'all' || values.target === 'web') {
    if (!existsSync(path.dirname(webFile))) {
      await mkdir(path.dirname(webFile), { recursive: true })
    }
    await writeFile(webFile, webSource(adapters))
  }
}

main().catch((error) => {
  console.error(`\nError de formularios: ${error.message}`)
  process.exitCode = 1
})
