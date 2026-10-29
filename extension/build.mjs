import { build } from 'esbuild'
import { cp, mkdir, rm } from 'node:fs/promises'
import { watch } from 'node:fs'

const watchMode = process.argv.includes('--watch')
const scripts = ['background.js', 'content.js', 'erp.js', 'popup.js', 'options.js']
const assets = ['manifest.json', 'popup.html', 'options.html', 'popup.css']

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

await run()

if (watchMode) {
  console.log('Watching src/ …')
  let timer
  watch('src', { recursive: true }, () => {
    clearTimeout(timer)
    timer = setTimeout(() => run().catch(console.error), 150)
  })
}
