// Regenerates the README screenshots in docs/images from a throwaway API, database and build.
// Usage: node scripts/screenshots/shoot.mjs
import { spawn, execFileSync } from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const backend = path.join(root, 'backend')
const frontend = path.join(root, 'frontend')
const images = path.join(root, 'docs/images')
const { chromium } = createRequire(path.join(frontend, 'package.json'))('playwright')

// The tool paths show on the settings page, so the folder has a short, fixed name. Each run clears it.
const tmp = '/tmp/goptimizer-demo'
fs.rmSync(tmp, { recursive: true, force: true })
fs.mkdirSync(tmp)
const children = []

// What `apt install pngquant optipng jpegoptim gifsicle` gives an Ubuntu server: oxipng has no package.
const tools = {
  pngquant: '2.18.0 (January 2023)',
  optipng: 'OptiPNG version 0.7.8',
  jpegoptim: 'jpegoptim v1.5.5  x86_64-pc-linux-gnu',
  gifsicle: 'LCDF Gifsicle 1.94',
}

const freePort = () =>
  new Promise((resolve) => {
    const server = net.createServer().listen(0, () => {
      const { port } = server.address()
      server.close(() => resolve(port))
    })
  })

async function waitFor(what, check, timeoutMs = 120_000) {
  const until = Date.now() + timeoutMs
  for (;;) {
    try {
      if (await check()) return
    } catch {
      // not up yet
    }
    if (Date.now() > until) throw new Error(`timed out waiting for ${what}`)
    await new Promise((resolve) => setTimeout(resolve, 500))
  }
}

function start(name, command, args, options) {
  const log = fs.openSync(path.join(tmp, `${name}.log`), 'a')
  children.push(spawn(command, args, { ...options, detached: true, stdio: ['ignore', log, log] }))
}

const run = (command, args, options) => execFileSync(command, args, { stdio: 'inherit', ...options })

const apiPort = await freePort()
const webPort = await freePort()
const web = `http://localhost:${webPort}`
const toolDir = path.join(tmp, 'usr/bin')
const env = {
  ...process.env,
  PATH: `${toolDir}:/usr/bin:/bin`,
  DATABASE_PATH: path.join(tmp, 'goptimizer.db'),
  JWT_SECRET: crypto.randomBytes(32).toString('hex'),
  APP_ENV: 'development',
  APP_DEBUG: 'true',
  AUTH_RATE_LIMIT: 'false',
  ENABLE_SCHEDULER: 'false',
  LOG_OUTPUT: 'stdout',
  LOG_FORMAT: 'text',
  CORS_ALLOWED_ORIGINS: '',
  MAIL_HOST: '',
}

async function main() {
  console.log(`Working in ${tmp}`)
  const bin = path.join(tmp, 'bin')
  for (const cmd of ['api', 'cli', 'screenshots-seed']) {
    run('go', ['build', '-o', path.join(bin, cmd), `./cmd/${cmd}`], { cwd: backend })
  }
  fs.mkdirSync(toolDir, { recursive: true })
  for (const [name, version] of Object.entries(tools)) {
    fs.writeFileSync(path.join(toolDir, name), `#!/bin/sh\necho '${version}'\n`, { mode: 0o755 })
  }

  // Run from tmp so neither binary reads backend/.env.
  run(path.join(bin, 'cli'), ['migrate', 'up'], { cwd: tmp, env })
  run(path.join(bin, 'screenshots-seed'), [], { cwd: tmp, env })
  start('api', path.join(bin, 'api'), ['--port', String(apiPort)], { cwd: tmp, env })
  await waitFor('the API', async () => (await fetch(`http://localhost:${apiPort}/health`)).ok)

  const dist = path.join(tmp, 'dist')
  run('npx', ['vite', 'build', '--outDir', dist, '--emptyOutDir', '--logLevel', 'warn'], {
    cwd: frontend,
    env: { ...process.env, VITE_API_URL: `http://localhost:${apiPort}/api/v1` },
  })
  start('web', 'npx', ['vite', 'preview', '--outDir', dist, '--port', String(webPort), '--strictPort'], {
    cwd: frontend,
    env: { ...process.env, VITE_API_URL: `http://localhost:${apiPort}/api/v1` },
  })
  await waitFor('the admin panel', async () => (await fetch(web)).ok)

  const browser = await chromium.launch()
  try {
    const signedOut = await page(browser, { width: 1280, height: 760 })
    await shoot(signedOut, '/', 'login', (p) => p.getByTestId('login-button').waitFor())

    // A short viewport with fullPage crops each shot to its content instead of empty background.
    const desktop = await signIn(await page(browser, { width: 1440, height: 400 }))
    const phone = await signIn(await page(browser, { width: 390, height: 844 }))
    const folderCards = (p) => p.locator('[data-testid^="folder-card-"]').nth(4).waitFor()

    await shoot(desktop, '/', 'dashboard', folderCards)
    await shoot(phone, '/', 'mobile-dashboard', folderCards, { fullPage: false })
    await shoot(desktop, '/folders/1/edit', 'folder-editor', (p) => p.getByTestId('folder-editor-form').waitFor())
    await shoot(desktop, '/settings', 'settings', (p) => p.getByTestId('tool-status-gifsicle').waitFor())
    await shoot(desktop, '/users', 'users', (p) => p.locator('[data-testid="users-table"] tr[data-row-key]').nth(4).waitFor())
  } finally {
    await browser.close()
  }
}

async function page(browser, viewport) {
  const context = await browser.newContext({ viewport, deviceScaleFactor: 2, reducedMotion: 'reduce', colorScheme: 'light', timezoneId: 'UTC', locale: 'en-GB' })
  await context.addInitScript(() => localStorage.setItem('i18nextLng', 'en'))
  return context.newPage()
}

async function signIn(p) {
  await p.goto(`${web}/`)
  await p.getByTestId('login-email-input').fill('admin@example.com')
  await p.getByTestId('login-password-input').fill('password123')
  await p.getByTestId('login-button').click()
  await p.getByTestId('main-content').waitFor()
  return p
}

async function shoot(p, url, name, ready, { fullPage = true } = {}) {
  await p.goto(web + url)
  await ready(p)
  await p.evaluate(() => document.fonts.ready)
  await p.mouse.move(0, 0)
  await p.screenshot({ path: path.join(images, `${name}.png`), fullPage })
  console.log(`docs/images/${name}.png`)
}

try {
  await main()
} finally {
  for (const child of children) {
    try {
      process.kill(-child.pid)
    } catch {
      // already gone
    }
  }
}
