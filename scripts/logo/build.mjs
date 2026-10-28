// Writes the wordmarks from docs/images/logo.svg and renders logo.png with the frontend's Playwright.
// Also renders the admin panel's PNG icons: favicon.png from frontend/public/favicon.svg (the mark
// redrawn with fewer, bolder shapes so it reads at 16px) and apple-touch-icon.png from the full mark.
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const images = path.join(root, 'docs/images')
const publicDir = path.join(root, 'frontend/public')
const { chromium } = createRequire(path.join(root, 'frontend/package.json'))('playwright')

const logo = fs.readFileSync(path.join(images, 'logo.svg'), 'utf8')
const mark = logo.replace(/^<svg[^>]*>/, '').replace(/<\/svg>\s*$/, '')

const wordmark = (ink) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 940 256" role="img" aria-label="Gogobackup">
  <svg x="0" y="0" width="256" height="256" viewBox="0 0 256 256">${mark}</svg>
  <text x="284" y="164" textLength="636" lengthAdjust="spacingAndGlyphs" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif" font-size="108" font-weight="700"><tspan fill="#00ADD8">Gogo</tspan><tspan fill="${ink}">backup</tspan></text>
</svg>
`
fs.writeFileSync(path.join(images, 'logo-wordmark.svg'), wordmark('#16313D'))
fs.writeFileSync(path.join(images, 'logo-wordmark-dark.svg'), wordmark('#E6EDF3'))

const dataUrl = (svg) => `data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}`

async function render(svg, size, out, { background, padding = 0 } = {}) {
  const page = await browser.newPage({ viewport: { width: size, height: size } })
  const inner = size - 2 * padding
  await page.setContent(
    `<body style="margin:0;background:${background ?? 'transparent'}"><img src="${dataUrl(svg)}" width="${inner}" height="${inner}" style="display:block;margin:${padding}px"></body>`
  )
  await page.screenshot({ path: out, omitBackground: !background })
  await page.close()
}

const browser = await chromium.launch()
await render(logo, 512, path.join(images, 'logo.png'))
await render(fs.readFileSync(path.join(publicDir, 'favicon.svg'), 'utf8'), 32, path.join(publicDir, 'favicon.png'))
// iOS fills transparency with black, so the touch icon gets a white tile.
await render(logo, 180, path.join(publicDir, 'apple-touch-icon.png'), { background: '#ffffff', padding: 14 })
await browser.close()
console.log('wrote logo-wordmark.svg, logo-wordmark-dark.svg, logo.png, favicon.png, apple-touch-icon.png')
