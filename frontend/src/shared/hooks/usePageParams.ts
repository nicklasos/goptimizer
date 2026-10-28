import { useCallback, useEffect, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { Page, PageParams } from '@/api/types'
import type { ServerPagination } from '@/shared/components/ResponsiveTable'

const MAX_PAGE_SIZE = 100

function positiveInt(value: string | null): number | null {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

export interface PageState {
  params: PageParams
  setPage: (page: number, pageSize: number) => void
}

/** Page and page size kept in the URL (`?page=&page_size=`), so reload and Back keep the position. */
export function usePageParams(defaultPageSize = 20): PageState {
  const [search, setSearch] = useSearchParams()
  const page = positiveInt(search.get('page')) ?? 1
  const pageSize = Math.min(positiveInt(search.get('page_size')) ?? defaultPageSize, MAX_PAGE_SIZE)

  const setPage = useCallback(
    (nextPage: number, nextPageSize: number) => {
      setSearch(
        (current) => {
          const next = new URLSearchParams(current)
          next.set('page', String(nextPage))
          next.set('page_size', String(nextPageSize))
          return next
        },
        { replace: true }
      )
    },
    [setSearch]
  )

  const params = useMemo(() => ({ page, page_size: pageSize }), [page, pageSize])
  return { params, setPage }
}

/**
 * Turns a paginated response into `ResponsiveTable` paging. Steps back a page
 * when the current one came back empty, which happens after deleting its last row.
 */
export function useTablePagination(state: PageState, data: Page<unknown> | undefined): ServerPagination | undefined {
  const { params, setPage } = state
  const isEmptyPage = data !== undefined && data.data.length === 0 && params.page > 1
  const lastPage = data?.pagination.last_page ?? 1

  useEffect(() => {
    if (isEmptyPage) setPage(Math.min(params.page - 1, lastPage), params.page_size)
  }, [isEmptyPage, lastPage, params.page, params.page_size, setPage])

  if (!data) return undefined
  return {
    current: params.page,
    pageSize: params.page_size,
    total: data.pagination.total,
    onChange: setPage,
  }
}
