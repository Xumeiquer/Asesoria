import { build } from 'esbuild'
import { cp, mkdir, rm } from 'node:fs/promises'
import { existsSync, watch } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const watchMode = process.argv.includes('--watch')
const scripts = ['background.js', 'content.js', 'erp.js', 'popup.js', 'options.js']
const assets = ['manifest.json', 'popup.html', 'options.html', 'popup.css']
const formsScript = fileURLToPath(new URL('../scripts/build-forms.mjs', import.meta.url))
const formsDir = fileURLToPath(new URL('../forms', import.meta.url))
const generatedFile = fileURLToPath(new URL('./src/adapters/generated.ts', import.meta.url))

function generateForms() {
  execFileSync(process.execPath, [formsScript, '--target', 'extension'], { stdio: 'inherit' })
}

async function run() {
  // Se genera también en la raíz para que cargar "extension/" a mano no deje
  // una extensión a medias. La carpeta recomendada sigue siendo extension/dist.
  for (const file of scripts) await rm(file, { force: true })
  await rm('dist', { recursive: true, force: true })
  await mkdir('dist', { recursive: true })
  await build({
    entryPoints: ['src/background.ts', 'src/content.ts', 'src/erp.ts', 'src/popup.ts', 'src/options.ts'],
    bundle: true,
    format: 'iife',
    target: ['chrome120', 'firefox121'],
    outdir: '.',
    entryNames: '[name]',
    sourcemap: false,
    logLevel: 'info',
  })
  for (const file of [...scripts, ...assets]) {
    await cp(file, `dist/${file}`)
  }
}

if (!existsSync(generatedFile)) generateForms()
await run()

if (watchMode) {
  console.log('Watching src/ and forms/ …')
  let timer
  watch('src', { recursive: true }, () => {
    clearTimeout(timer)
    timer = setTimeout(() => run().catch(console.error), 150)
  })
  watch(formsDir, { recursive: true }, () => {
    clearTimeout(timer)
    timer = setTimeout(() => {
      try {
        generateForms()
        void run().catch(console.error)
      } catch (error) {
        console.error(error)
      }
    }, 150)
  })
}
