import type { ReactNode } from 'react'
import { Card, Grid, List, Table, Typography, theme } from 'antd'
import type { ColumnsType } from 'antd/es/table'

/** Server-side paging: the items are one page, and changing the page refetches. */
export interface ServerPagination {
  current: number
  pageSize: number
  total: number
  onChange: (page: number, pageSize: number) => void
}

interface Props<T extends { id: number }> {
  testId: string
  items: T[] | undefined
  loading?: boolean
  columns: ColumnsType<T>
  emptyText: string
  /** Card title on phones (bold, left). */
  cardTitle?: (item: T) => ReactNode
  /** Right side of the card title row: a state tag or the main figure. */
  cardExtra?: (item: T) => ReactNode
  /** Card body under the title row. */
  renderCard?: (item: T) => ReactNode
  cardActions?: (item: T) => ReactNode
  /** Makes rows and cards open the item. */
  onItemClick?: (item: T) => void
  /** Test id per card on phones (rows keep AntD's `data-row-key`). */
  cardTestId?: (item: T) => string
  /** Omit to page the given items in the browser, `false` for no paging, or pass server-side paging. */
  pagination?: false | ServerPagination
}

/** A table on wide screens (`lg` and up), a list of cards below that. */
export function ResponsiveTable<T extends { id: number }>({
  testId,
  items,
  loading,
  columns,
  emptyText,
  cardTitle,
  cardExtra,
  renderCard,
  cardActions,
  onItemClick,
  cardTestId,
  pagination,
}: Props<T>) {
  const screens = Grid.useBreakpoint()
  const { token } = theme.useToken()
  const data = items ?? []
  const serverPagination = pagination || undefined

  if (!screens.lg) {
    return (
      <List
        data-testid={testId}
        dataSource={data}
        loading={loading}
        locale={{ emptyText }}
        pagination={serverPagination ? { ...serverPagination, size: 'small', align: 'center' } : undefined}
        renderItem={(item) => (
          <Card
            key={item.id}
            size="small"
            hoverable={Boolean(onItemClick)}
            onClick={onItemClick ? () => onItemClick(item) : undefined}
            data-testid={cardTestId?.(item)}
            style={{ marginBottom: token.marginSM }}
            actions={cardActions ? [cardActions(item)] : undefined}
          >
            {(cardTitle || cardExtra) && (
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: token.marginSM }}>
                <Typography.Text strong style={{ minWidth: 0 }}>
                  {cardTitle?.(item)}
                </Typography.Text>
                {cardExtra && <div style={{ flexShrink: 0 }}>{cardExtra(item)}</div>}
              </div>
            )}
            {renderCard && <div style={cardTitle || cardExtra ? { marginTop: token.marginXXS } : undefined}>{renderCard(item)}</div>}
          </Card>
        )}
      />
    )
  }

  return (
    <Table<T>
      data-testid={testId}
      columns={columns}
      dataSource={data}
      rowKey="id"
      loading={loading}
      locale={{ emptyText }}
      pagination={
        pagination === false
          ? false
          : serverPagination
            ? { ...serverPagination, showSizeChanger: true }
            : { showSizeChanger: true, hideOnSinglePage: true }
      }
      scroll={{ x: 'max-content' }}
      onRow={onItemClick ? (item) => ({ onClick: () => onItemClick(item), style: { cursor: 'pointer' } }) : undefined}
    />
  )
}
