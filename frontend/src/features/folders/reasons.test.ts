import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import i18next from 'i18next'
import en from '@/locales/en/translation.json' with { type: 'json' }
import uk from '@/locales/uk/translation.json' with { type: 'json' }
import { setFormatLanguage } from '@/shared/utils/format'
import type { Translate } from '@/shared/utils/serverErrors'
import { explainImage, explainReason, REASON_CODES } from './reasons'
import type { Image, ReasonCode } from './types'

type Tree = { [key: string]: string | Tree }

function lookup(tree: Tree, key: string): unknown {
  return key.split('.').reduce<unknown>((node, part) => (node && typeof node === 'object' ? (node as Tree)[part] : undefined), tree)
}

const i18n = i18next.createInstance()
await i18n.init({ lng: 'en', resources: { en: { translation: en } }, interpolation: { escapeValue: false } })
const t = i18n.t.bind(i18n) as Translate
setFormatLanguage('en')

const image = (overrides: Partial<Image>): Image =>
  ({ status: 'kept', reason: null, lossless_reason: null, attempts: 0, next_attempt_at: null, ...overrides }) as Image

describe('REASON_CODES', () => {
  for (const [language, tree] of [['en', en], ['uk', uk]] as const) {
    it(`words every reason code in ${language}`, () => {
      const missing = Object.entries(REASON_CODES)
        .filter(([, key]) => {
          const sentence = lookup(tree as Tree, key)
          return typeof sentence !== 'string' || sentence.trim() === ''
        })
        .map(([code]) => code)
      assert.deepEqual(missing, [])
    })
  }

  it('gives every code a sentence of its own', () => {
    const sentences = Object.values(REASON_CODES).map((key) => lookup(en as Tree, key))
    assert.equal(new Set(sentences).size, sentences.length)
  })
})

describe('explainReason', () => {
  it('words a known code and keeps what was measured', () => {
    const reason = explainReason(t, { code: 'ssim_below_threshold', detail: '0.7100, below 0.8500, at 128,64' })
    assert.deepEqual(reason, {
      code: 'ssim_below_threshold',
      text: lookup(en as Tree, REASON_CODES.ssim_below_threshold),
      detail: '0.7100, below 0.8500, at 128,64',
    })
  })

  it('shows text with no code it knows as it was stored', () => {
    assert.deepEqual(explainReason(t, { code: null, detail: 'quantum flux: 42' }), { code: null, text: 'quantum flux: 42', detail: null })
  })

  it('is nothing without a reason', () => {
    assert.equal(explainReason(t, null), null)
    assert.equal(explainReason(t, { code: null, detail: null }), null)
  })
})

describe('explainImage', () => {
  it('words the refused lossy result and, on its own line, the lossless attempt after it', () => {
    const { reason, lossless } = explainImage(
      t,
      image({
        reason: { code: 'ssim_below_threshold', detail: '0.7100, below 0.8500, at 128,64' },
        lossless_reason: { code: 'no_saving' as ReasonCode, detail: '1.2% smaller, below 3%' },
      })
    )
    assert.equal(reason?.code, 'ssim_below_threshold')
    const noSaving = lookup(en as Tree, REASON_CODES.no_saving) as string
    assert.equal(lossless?.code, 'no_saving')
    assert.ok(lossless?.text.includes(noSaving), lossless?.text)
    assert.notEqual(lossless?.text, noSaving, 'the lossless line says it was the lossless attempt')
    assert.equal(lossless?.detail, '1.2% smaller, below 3%')
  })

  it('has no lossless line when there was no lossless attempt', () => {
    assert.equal(explainImage(t, image({ reason: { code: 'apng', detail: null } })).lossless, null)
  })

  it('says when a failed image is tried again', () => {
    const now = new Date('2026-10-29T12:00:00Z')
    const { retry } = explainImage(t, image({ status: 'failed', attempts: 1, next_attempt_at: '2026-10-29T12:10:00Z' }), now)
    assert.ok(retry?.includes('in 10 minutes'), retry ?? 'no retry line')
  })

  it('says a failed image is no longer tried, after how many attempts', () => {
    const { retry } = explainImage(t, image({ status: 'failed', attempts: 3, next_attempt_at: null }))
    assert.ok(retry?.includes('3'), retry ?? 'no retry line')
    assert.notEqual(retry, explainImage(t, image({ status: 'failed', attempts: 1, next_attempt_at: '2026-10-29T12:10:00Z' })).retry)
  })

  it('has no retry line for an image that did not fail', () => {
    assert.equal(explainImage(t, image({ status: 'kept', attempts: 3 })).retry, null)
  })
})
