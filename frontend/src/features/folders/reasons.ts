import { formatRelative } from '@/shared/utils/format'
import type { Translate } from '@/shared/utils/serverErrors'
import type { Image, Reason, ReasonCode } from './types'

/**
 * The sentence each reason code is worded by, in the locales. Keyed by the API's union, so a code
 * the API adds fails `tsc` here until it has its sentence; reasons.test.ts checks every sentence is
 * in each language.
 */
export const REASON_CODES = {
  symlink: 'images.reasons.symlink',
  not_regular_file: 'images.reasons.not_regular_file',
  hard_link: 'images.reasons.hard_link',
  access_control_list: 'images.reasons.access_control_list',
  wrong_content: 'images.reasons.wrong_content',
  apng: 'images.reasons.apng',
  extra_data: 'images.reasons.extra_data',
  too_many_pixels: 'images.reasons.too_many_pixels',
  too_large: 'images.reasons.too_large',
  unreadable: 'images.reasons.unreadable',
  already_optimized: 'images.reasons.already_optimized',
  too_new: 'images.reasons.too_new',
  changed_while_optimizing: 'images.reasons.changed_while_optimizing',
  interrupted: 'images.reasons.interrupted',
  tool_missing: 'images.reasons.tool_missing',
  tool_failed: 'images.reasons.tool_failed',
  swap_failed: 'images.reasons.swap_failed',
  no_saving: 'images.reasons.no_saving',
  format_changed: 'images.reasons.format_changed',
  result_undecodable: 'images.reasons.result_undecodable',
  dimensions_changed: 'images.reasons.dimensions_changed',
  frames_changed: 'images.reasons.frames_changed',
  animation_changed: 'images.reasons.animation_changed',
  pixels_changed: 'images.reasons.pixels_changed',
  ssim_below_threshold: 'images.reasons.ssim_below_threshold',
  color_changed: 'images.reasons.color_changed',
  metadata_changed: 'images.reasons.metadata_changed',
} as const satisfies Record<ReasonCode, string>

/** A reason as the admin reads it: a sentence, and what was measured (in English) under it. */
export interface ReasonText {
  code: ReasonCode | null
  text: string
  detail: string | null
}

/** A known code reads as its sentence; text the API could not match to a code is shown as it was stored. */
export function explainReason(t: Translate, reason: Reason | null | undefined): ReasonText | null {
  if (!reason) return null
  const detail = reason.detail || null
  if (!reason.code) return detail ? { code: null, text: detail, detail: null } : null
  return { code: reason.code, text: t(REASON_CODES[reason.code]), detail }
}

export interface ImageExplanation {
  reason: ReasonText | null
  /** How the lossless attempt after a refused lossy result ended. */
  lossless: ReasonText | null
  /** For a failed image: when it is tried again, or that it no longer is. */
  retry: string | null
}

/** Why an image is as it is, in plain words. */
export function explainImage(
  t: Translate,
  image: Pick<Image, 'status' | 'reason' | 'lossless_reason' | 'attempts' | 'next_attempt_at'>,
  now: Date = new Date()
): ImageExplanation {
  const lossless = explainReason(t, image.lossless_reason)
  let retry: string | null = null
  if (image.status === 'failed') {
    retry = image.next_attempt_at
      ? t('images.retryAt', { when: formatRelative(image.next_attempt_at, now) })
      : t('images.gaveUp', { count: image.attempts })
  }
  return {
    reason: explainReason(t, image.reason),
    lossless: lossless && { ...lossless, text: t('images.losslessAttempt', { reason: lossless.text }) },
    retry,
  }
}
