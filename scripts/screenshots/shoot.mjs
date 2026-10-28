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

// The plan's source path and the local storage's folder show in the run log, the storage browser
// and the restore form, and the API needs them to be real, so the folder gets a short, fixed name
// instead of a random one. A run starts by clearing what the last one left.
const tmp = '/tmp/gogobackup-demo'
fs.rmSync(tmp, { recursive: true, force: true })
fs.mkdirSync(tmp)
const children = []

const freePort = () =>
  new Promise((resolve) => {
    const server = net.createServer().listen(0, () => {
      const { port } = server.address()
      server.close(() => resolve(port))
    })
  })

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function waitFor(what, check, timeoutMs = 120_000) {
  const until = Date.now() + timeoutMs
  for (;;) {
    try {
      const value = await check()
      if (value) return value
    } catch {
      // not up yet
    }
    if (Date.now() > until) throw new Error(`timed out waiting for ${what}`)
    await sleep(500)
  }
}

function start(name, command, args, options) {
  const log = fs.openSync(path.join(tmp, `${name}.log`), 'a')
  const child = spawn(command, args, { ...options, detached: true, stdio: ['ignore', log, log] })
  children.push(child)
  return child
}

function run(command, args, options) {
  execFileSync(command, args, { stdio: 'inherit', ...options })
}

const apiPort = await freePort()
const webPort = await freePort()
const api = `http://localhost:${apiPort}/api/v1`
const web = `http://localhost:${webPort}`
const nas = path.join(tmp, 'nas')
const analytics = path.join(tmp, 'analytics.db')
const env = {
  ...process.env,
  DATABASE_PATH: path.join(tmp, 'gogobackup.db'),
  ENCRYPTION_KEY: crypto.randomBytes(32).toString('base64'),
  JWT_SECRET: crypto.randomBytes(32).toString('hex'),
  APP_ENV: 'development',
  APP_DEBUG: 'true',
  AUTH_RATE_LIMIT: 'false',
  ENABLE_SCHEDULER: 'true',
  WORK_DIR: path.join(tmp, 'work'),
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

  run('sqlite3', [
    analytics,
    `CREATE TABLE events (id INTEGER PRIMARY KEY, at TEXT, name TEXT, path TEXT, visitor TEXT);
     WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 120000)
     INSERT INTO events (at, name, path, visitor)
     SELECT datetime('2026-09-01', '+' || (i * 23) || ' seconds'),
            CASE i % 4 WHEN 0 THEN 'pageview' WHEN 1 THEN 'click' WHEN 2 THEN 'signup' ELSE 'checkout' END,
            '/products/' || (i * 7919 % 1500), hex(randomblob(8))
     FROM n;`,
  ])

  // Run from tmp so neither binary reads backend/.env.
  run(path.join(bin, 'cli'), ['migrate', 'up'], { cwd: tmp, env })
  run(path.join(bin, 'screenshots-seed'), ['-phase', 'history', '-nas', nas, '-analytics', analytics], { cwd: tmp, env })

  start('api', path.join(bin, 'api'), ['--port', String(apiPort)], { cwd: tmp, env })
  await waitFor('the API', async () => (await fetch(`http://localhost:${apiPort}/health`)).ok)

  const dist = path.join(tmp, 'dist')
  run('npx', ['vite', 'build', '--outDir', dist, '--emptyOutDir', '--logLevel', 'warn'], {
    cwd: frontend,
    env: { ...process.env, VITE_API_BASE_URL: `http://localhost:${apiPort}` },
  })
  start('web', 'npx', ['vite', 'preview', '--outDir', dist, '--port', String(webPort), '--strictPort'], { cwd: frontend })
  await waitFor('the admin panel', async () => (await fetch(web)).ok)

  const login = await post('/auth/login', { email: 'admin@example.com', password: 'password123' })
  const token = login.access_token
  const call = async (method, url, body) => {
    const response = await fetch(api + url, {
      method,
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: body && JSON.stringify(body),
    })
    if (!response.ok) throw new Error(`${method} ${url}: ${response.status} ${await response.text()}`)
    return (await response.json()).data
  }
  const planId = async (name) => (await call('GET', '/plans?page_size=100')).find((plan) => plan.name === name).id

  const analyticsId = await planId('analytics SQLite')
  const queued = await call('POST', `/plans/${analyticsId}/run`)
  const backup = await waitFor('the analytics backup', async () => {
    const r = await call('GET', `/runs/${queued.id}`)
    if (r.status === 'failed') throw new Error(`the analytics backup failed: ${r.error}`)
    return r.status === 'succeeded' && r
  })
  const [archive] = await call('GET', `/plans/${analyticsId}/archives`)

  // The live phase adds an overdue plan, which the scheduler queues at the start of each minute,
  // so it goes in right after one and the dashboard is shot before the next.
  await sleep((62 - new Date().getSeconds()) * 1000)
  run(path.join(bin, 'screenshots-seed'), ['-phase', 'live', '-nas', nas], { cwd: tmp, env })
  const minuteEnds = Date.now() + (58 - new Date().getSeconds()) * 1000

  const browser = await chromium.launch()
  try {
    const desktop = await signedIn(browser, { width: 1440, height: 900 })
    const phone = await signedIn(browser, { width: 390, height: 844 })

    await shoot(desktop, '/', 'dashboard', (page) => cardsLoaded(page, 6), { clip: await belowCards(desktop) })
    await shoot(phone, '/', 'mobile-dashboard', (page) => cardsLoaded(page, 1))
    if (Date.now() > minuteEnds) throw new Error('the dashboard shots took past the scheduler tick; run again')

    const shopId = await planId('shop-prod Postgres')
    await shoot(desktop, `/plans/${shopId}`, 'plan-runs', (page) => page.getByTestId('plan-runs-chart').waitFor())
    await shoot(desktop, `/plans/${analyticsId}?run=${backup.id}`, 'run-log', async (page) => {
      await page.getByTestId('run-log').getByText('Archive:').waitFor()
    })
    await shoot(desktop, `/plans/${shopId}/edit`, 'plan-editor', (page) => page.getByTestId('plan-schedules').waitFor(), { fullPage: true })

    const storages = await call('GET', '/storages?page_size=100')
    const nasId = storages.find((storage) => storage.name === 'NAS (local)').id
    await shoot(desktop, `/storages/${nasId}/browse`, 'storage-browser', async (page) => {
      await page.getByTestId('storage-folder-shop-prod-postgres').click()
      await page.getByTestId('storage-crumb-shop-prod-postgres').waitFor()
      await page.locator('[data-testid^="storage-file-"]').first().waitFor()
    })

    await shoot(desktop, `/plans/${analyticsId}?tab=archives`, 'restore', async (page) => {
      await page.getByTestId(`archive-restore-button-${archive.id}`).click()
      const confirm = page.getByTestId('restore-confirm-input')
      await confirm.waitFor()
      await confirm.fill(analytics)
    })
  } finally {
    await browser.close()
  }
}

async function post(url, body) {
  const response = await fetch(api + url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  if (!response.ok) throw new Error(`POST ${url}: ${response.status} ${await response.text()}`)
  const json = await response.json()
  return json.data ?? json
}

async function signedIn(browser, viewport) {
  const context = await browser.newContext({ viewport, deviceScaleFactor: 2, reducedMotion: 'reduce', colorScheme: 'light', timezoneId: 'UTC', locale: 'en-GB' })
  await context.addInitScript(() => localStorage.setItem('i18nextLng', 'en'))
  const page = await context.newPage()
  await page.goto(`${web}/login`)
  await page.getByTestId('login-email-input').fill('admin@example.com')
  await page.getByTestId('login-password-input').fill('password123')
  await page.getByTestId('login-button').click()
  await page.getByTestId('main-content').waitFor()
  return page
}

// belowCards is the top of the dashboard down to just under its cards, without the empty rest.
async function belowCards(page) {
  await page.goto(web + '/')
  await cardsLoaded(page, 6)
  const cards = await page.getByTestId('dashboard-cards').boundingBox()
  const { width, height } = page.viewportSize()
  return { x: 0, y: 0, width, height: Math.min(height, Math.ceil(cards.y + cards.height + 32)) }
}

async function cardsLoaded(page, atLeast) {
  await page.getByTestId('dashboard-cards').waitFor()
  await waitFor('the plan cards', async () => (await page.locator('[data-testid^="plan-card-name-"]').count()) >= atLeast, 15_000)
}

async function shoot(page, url, name, ready, options = {}) {
  await page.goto(web + url)
  await ready(page)
  await page.waitForLoadState('networkidle')
  await waitFor('spinners to go', async () => (await page.locator('.ant-spin-spinning, .ant-skeleton').count()) === 0, 15_000)
  await page.mouse.move(0, 0)
  await sleep(400)
  const shot = await page.screenshot({ animations: 'disabled', caret: 'hide', ...options })
  const out = path.join(images, `${name}.png`)
  fs.writeFileSync(out, await halve(page, shot))
  console.log(`${path.relative(root, out)} ${Math.round(fs.statSync(out).size / 1024)} KB`)
}

// halve renders a deviceScaleFactor 2 capture at its CSS size, which antialiases text better
// than capturing at 1x.
async function halve(page, png) {
  const scratch = await page.context().browser().newPage()
  try {
    const base64 = await scratch.evaluate(async (data) => {
      const blob = await (await fetch(`data:image/png;base64,${data}`)).blob()
      const full = await createImageBitmap(blob)
      const width = Math.round(full.width / 2)
      const height = Math.round(full.height / 2)
      const small = await createImageBitmap(blob, { resizeWidth: width, resizeHeight: height, resizeQuality: 'high' })
      const canvas = new OffscreenCanvas(width, height)
      canvas.getContext('2d').drawImage(small, 0, 0)
      const out = await canvas.convertToBlob({ type: 'image/png' })
      const bytes = new Uint8Array(await out.arrayBuffer())
      let binary = ''
      for (const byte of bytes) binary += String.fromCharCode(byte)
      return btoa(binary)
    }, png.toString('base64'))
    return Buffer.from(base64, 'base64')
  } finally {
    await scratch.close()
  }
}

let failed = false
try {
  await main()
} catch (error) {
  failed = true
  console.error(error)
  console.error(`The API and preview logs are in ${tmp}`)
} finally {
  for (const child of children) {
    try {
      process.kill(-child.pid, 'SIGTERM')
    } catch {
      // already gone
    }
  }
  await sleep(1500)
  if (!failed) fs.rmSync(tmp, { recursive: true, force: true })
  process.exitCode = failed ? 1 : 0
}
