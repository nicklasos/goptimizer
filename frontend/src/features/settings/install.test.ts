import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { installCommands } from './install'

describe('installCommands', () => {
  it('gives the apt and brew commands', () => {
    assert.deepEqual(installCommands({ apt: 'pngquant', brew: 'pngquant' }), {
      apt: 'sudo apt install pngquant',
      brew: 'brew install pngquant',
    })
  })

  it('has no apt command for a program Ubuntu does not package', () => {
    assert.equal(installCommands({ apt: '', brew: 'oxipng' }).apt, null)
  })
})
