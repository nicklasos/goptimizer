import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import en from './en/translation.json' with { type: 'json' }
import uk from './uk/translation.json' with { type: 'json' }

type Tree = { [key: string]: string | Tree }

// Plural forms differ by language (English has two, Ukrainian four), so they count as one key.
const PLURAL_SUFFIX = /_(zero|one|two|few|many|other)$/

function keys(tree: Tree, prefix = ''): Set<string> {
  const out = new Set<string>()
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') out.add(path.replace(PLURAL_SUFFIX, ''))
    else for (const nested of keys(value, path)) out.add(nested)
  }
  return out
}

function emptyValues(tree: Tree, prefix = ''): string[] {
  return Object.entries(tree).flatMap(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') return value.trim() === '' ? [path] : []
    return emptyValues(value, path)
  })
}

describe('locales', () => {
  const english = keys(en as Tree)
  const ukrainian = keys(uk as Tree)

  it('every English key has a Ukrainian translation', () => {
    assert.deepEqual([...english].filter((key) => !ukrainian.has(key)), [])
  })

  it('every Ukrainian key has an English translation', () => {
    assert.deepEqual([...ukrainian].filter((key) => !english.has(key)), [])
  })

  it('no translation is empty', () => {
    assert.deepEqual(emptyValues(en as Tree), [])
    assert.deepEqual(emptyValues(uk as Tree), [])
  })
})
