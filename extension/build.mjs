import { build } from 'esbuild'
import { cp, mkdir, rm } from 'node:fs/promises'
import { watch } from 'node:fs'

const watchMode = process.argv.includes('--watch')

async function run() {
  await rm('dist', { recursive: true, force: true })
  await mkdir('dist', { recursive: true })
  await build({
    entryPoints: ['src/background.ts', 'src/content.ts', 'src/popup.ts', 'src/options.ts'],
    bundle: true,
    format: 'iife',
    target: ['chrome120', 'firefox121'],
    outdir: 'dist',
    entryNames: '[name]',
    sourcemap: false,
    logLevel: 'info',
  })
  for (const file of ['manifest.json', 'popup.html', 'options.html', 'popup.css']) {
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
