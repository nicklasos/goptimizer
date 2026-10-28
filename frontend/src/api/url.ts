export type QueryValue = string | number | boolean | null | undefined
export type Query = Record<string, QueryValue>

/** Joins base, path and query; empty, null and undefined query values are left out. */
export function buildUrl(base: string, path: string, query?: Query): string {
  const url = `${base}${path}`
  if (!query) return url
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    params.append(key, String(value))
  }
  const qs = params.toString()
  return qs ? `${url}?${qs}` : url
}
