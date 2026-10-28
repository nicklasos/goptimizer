import fs from 'fs'
import os from 'os'
import path from 'path'
import { test, expect } from '@playwright/test'
import { loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { StorageBrowserPage } from '../pages/StorageBrowserPage'
import { StorageEditorPage } from '../pages/StorageEditorPage'
import { StoragesPage } from '../pages/StoragesPage'

/** A directory the API (same machine) can write, with one file at the top and one in a folder. */
function backupDir(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gogobackup-e2e-'))
  fs.writeFileSync(path.join(dir, 'readme.txt'), 'hello')
  fs.mkdirSync(path.join(dir, 'nightly'))
  fs.writeFileSync(path.join(dir, 'nightly', 'db-2026-10-17.dump.gz'), 'x'.repeat(2048))
  return dir
}

test.describe('Storages as an admin', () => {
  test('adds a local storage, tests it, browses it, edits it and deletes it', async ({ page }) => {
    await loginAs(page, 'admin')
    const dir = backupDir()
    const name = `E2E Local ${Date.now()}`
    const list = new StoragesPage(page)
    const editor = new StorageEditorPage(page)
    const browser = new StorageBrowserPage(page)

    await list.gotoList()
    await list.clickAdd()
    await editor.nameInput.fill(name)
    await editor.pathInput.fill(path.join(dir, 'does-not-exist'))
    await editor.testConnection()
    await expect(editor.testFailed).toHaveAttribute('data-step', 'write')

    await editor.pathInput.fill(dir)
    await editor.testConnection()
    await expect(editor.testPassed).toBeVisible()
    await editor.save()

    await expect(list.row(name)).toBeVisible()
    await list.browse(name)
    await expect(browser.rows).toHaveCount(2)
    await expect(browser.rows.first()).toContainText('nightly')
    await expect(browser.file('readme.txt')).toBeVisible()

    await browser.folder('nightly').click()
    await expect(page).toHaveURL(/prefix=nightly%2F/)
    await expect(browser.file('db-2026-10-17.dump.gz')).toBeVisible()
    await expect(browser.rows).toHaveCount(1)
    await browser.crumb('root').click()
    await expect(browser.file('readme.txt')).toBeVisible()

    await list.gotoList()
    await list.edit(name)
    await expect(editor.pathInput).toHaveValue(dir)
    await editor.nameInput.fill(`${name} renamed`)
    await editor.save()
    await expect(list.row(`${name} renamed`)).toBeVisible()

    await list.delete(`${name} renamed`)
    fs.rmSync(dir, { recursive: true, force: true })
  })

  test('shows the fields of each kind and marks what is missing', async ({ page }) => {
    await loginAs(page, 'admin')
    const list = new StoragesPage(page)
    const editor = new StorageEditorPage(page)

    await list.gotoList()
    await list.clickAdd()
    await editor.nameInput.fill(`E2E Cloud ${Date.now()}`)

    await editor.chooseKind('s3')
    await expect(editor.pathInput).toHaveCount(0)
    await editor.testButton.click()
    await expect(editor.s3BucketInput).toHaveAttribute('aria-invalid', 'true')
    await expect(editor.s3SecretKeyInput).toHaveAttribute('aria-invalid', 'true')

    await editor.chooseKind('gcs')
    await expect(editor.s3BucketInput).toHaveCount(0)
    await editor.saveButton.click()
    await expect(editor.gcsCredentialsInput).toHaveAttribute('aria-invalid', 'true')

    await editor.chooseKind('local')
    await editor.pathInput.fill('relative/path')
    await editor.saveButton.click()
    await expect(editor.pathInput).toHaveAttribute('aria-invalid', 'true')
  })
})

test.describe('Storages as a plain user', () => {
  test('sees no storage menu and is refused on the pages', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)
    const list = new StoragesPage(page)

    await shell.goto('/')
    await expect(shell.navItem('dashboard')).toBeVisible()
    await expect(shell.navItem('storages')).toHaveCount(0)

    for (const target of ['/storages', '/storages/new', '/storages/1/browse']) {
      await list.goto(target)
      await expect(list.forbidden).toBeVisible()
    }
  })
})
