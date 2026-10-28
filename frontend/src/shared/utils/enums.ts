import type { Translate } from './serverErrors'

/** The label of an enum value: `enums.<enumName>.<value>` in the locales, or the raw value. */
export function enumLabel(t: Translate, enumName: string, value: string): string {
  return t(`enums.${enumName}.${value}`, { defaultValue: value })
}
