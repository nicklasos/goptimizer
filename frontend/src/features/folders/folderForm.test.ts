import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { folderToForm, formToRequest, NEW_FOLDER } from './folderForm'
import type { Folder } from './types'

const folder: Folder = {
  id: 7,
  name: 'Shop',
  kind: 'local',
  path: '/srv/shop/uploads',
  enabled: false,
  scan_interval_minutes: 15,
  full_scan_interval_hours: 48,
  profile: { ...NEW_FOLDER.profile, mode: 'lossless', excludes: ['cache/**'] },
  created_at: '2026-10-28T10:00:00Z',
  updated_at: '2026-10-28T10:00:00Z',
}

describe('folder form', () => {
  it('edits what a save sends and sends it back unchanged', () => {
    assert.deepEqual(formToRequest(folderToForm(folder)), {
      kind: 'local',
      name: 'Shop',
      path: '/srv/shop/uploads',
      scan_interval_minutes: 15,
      full_scan_interval_hours: 48,
      profile: folder.profile,
    })
  })

  it('trims the name and the path', () => {
    const request = formToRequest({ ...NEW_FOLDER, name: '  Blog ', path: ' /srv/blog\t' })
    assert.equal(request.name, 'Blog')
    assert.equal(request.path, '/srv/blog')
  })
})
