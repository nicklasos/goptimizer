export interface Crumb {
  name: string
  prefix: string
}

/** The folders above and including `prefix` ("a/b/"): [{ a, "a/" }, { b, "a/b/" }]. The root is not included. */
export function prefixCrumbs(prefix: string): Crumb[] {
  const crumbs: Crumb[] = []
  let path = ''
  for (const name of prefix.split('/').filter(Boolean)) {
    path += `${name}/`
    crumbs.push({ name, prefix: path })
  }
  return crumbs
}

export function childPrefix(prefix: string, folder: string): string {
  return `${normalizePrefix(prefix)}${folder}/`
}

/** "" for the root, otherwise the folder path with one trailing slash. */
export function normalizePrefix(prefix: string | null | undefined): string {
  const parts = (prefix ?? '').split('/').filter(Boolean)
  return parts.length ? `${parts.join('/')}/` : ''
}
