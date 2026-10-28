/** Has the browser download a URL that answers with Content-Disposition: attachment, without leaving the page. */
export function startDownload(url: string): void {
  const link = document.createElement('a')
  link.href = url
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
}
