import { useEffect, useRef, useState } from 'react'
import { Button, Card, Input, Select, Space, Tabs, Tag, Tooltip, Typography, theme } from 'antd'
import { RedoOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { enumLabel } from '@/shared/utils/enums'
import { EMPTY, formatBytes, formatCount, formatDateTime, formatNumber, formatRelative } from '@/shared/utils/format'
import message from '@/shared/utils/message'
import { useFolderImages, useRetryFailedImages, useRetryImage } from '../hooks'
import { IMAGE_STATUS, IMAGE_STATUSES, imageSizes, isImageStatus, readImageFilters, writeImageFilters, type ImageFilters } from '../images'
import { explainImage, type ReasonText } from '../reasons'
import { imagesIn } from '../scans'
import type { Folder, Image, ImageOrder } from '../types'

const SEARCH_DELAY = 300
const PATH_WIDTH = 360
const REASON_WIDTH = 420

/** A folder's images by status, searchable by path, with a retry for the failed ones. The filters live in the URL. */
export function FolderImages({ folder }: { folder: Folder }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const [search, setSearch] = useSearchParams()
  const filters = readImageFilters(search)
  const images = useFolderImages(folder.id, { ...filters, q: filters.q.trim() })
  const [text, setText] = useState(filters.q)
  const typing = useRef<ReturnType<typeof setTimeout>>(undefined)
  const retry = useRetryImage()
  const retryAll = useRetryFailedImages()
  const items = images.data?.pages.flatMap((page) => page.items)
  const failed = imagesIn(folder, 'failed')
  const searching = filters.q.trim() !== ''

  // React Router hands an updater the params of the render it came from, which can be one change behind
  // (a tab clicked just before the search box fires). The address bar is already current.
  const setFilters = (change: Partial<ImageFilters>) =>
    setSearch(writeImageFilters(new URLSearchParams(window.location.search), change), { replace: true })

  // The URL, and the query with it, follows the search box once typing pauses: on a big folder a search reads the whole path index.
  const onSearch = (value: string) => {
    setText(value)
    clearTimeout(typing.current)
    typing.current = setTimeout(() => setFilters({ q: value }), SEARCH_DELAY)
  }
  useEffect(() => () => clearTimeout(typing.current), [])

  const tabs = [
    { key: 'all', label: t('images.all'), count: Object.values(folder.image_counts).reduce((sum, count) => sum + count, 0) },
    ...IMAGE_STATUSES.map((status) => ({ key: status, label: enumLabel(t, 'imageStatus', status), count: imagesIn(folder, status) })),
  ]

  const statusTag = (image: Image) => (
    <Tag color={IMAGE_STATUS[image.status].color} data-testid={`image-status-${image.id}`} data-status={image.status}>
      {enumLabel(t, 'imageStatus', image.status)}
    </Tag>
  )

  const retryButton = (image: Image) =>
    image.status === 'failed' && (
      <Button
        size="small"
        icon={<RedoOutlined />}
        loading={retry.isPending && retry.variables === image.id}
        data-testid={`retry-image-button-${image.id}`}
        onClick={() => retry.mutate(image.id, { onSuccess: () => message.success(t('images.retriedImage')) })}
      >
        {t('images.retry')}
      </Button>
    )

  const updated = (image: Image) => (
    <Tooltip title={formatDateTime(image.updated_at)}>
      <span>{formatRelative(image.updated_at)}</span>
    </Tooltip>
  )

  const columns: ColumnsType<Image> = [
    { title: t('images.path'), key: 'path', render: (_, image) => <ImagePath path={image.path} maxWidth={PATH_WIDTH} /> },
    { title: t('images.format'), dataIndex: 'format', key: 'format', render: (format: Image['format']) => format.toUpperCase() },
    { title: t('images.size'), key: 'size', render: (_, image) => <ImageSize image={image} /> },
    { title: t('images.status'), key: 'status', render: (_, image) => statusTag(image) },
    { title: t('images.reason'), key: 'reason', render: (_, image) => <ImageReason image={image} maxWidth={REASON_WIDTH} /> },
    { title: t('images.updated'), key: 'updated', render: (_, image) => updated(image) },
    { key: 'actions', render: (_, image) => retryButton(image) },
  ]

  return (
    <Card
      title={t('images.title')}
      data-testid="folder-images"
      extra={
        failed > 0 && (
          <Button
            icon={<RedoOutlined />}
            loading={retryAll.isPending}
            data-testid="retry-failed-images-button"
            data-count={failed}
            onClick={() => retryAll.mutate(folder.id, { onSuccess: ({ retried }) => message.success(t('images.retried', { count: retried })) })}
          >
            {t('images.retryAll', { count: failed })}
          </Button>
        )
      }
    >
      <Tabs
        activeKey={filters.status ?? 'all'}
        onChange={(key) => setFilters({ status: isImageStatus(key) ? key : null })}
        items={tabs.map(({ key, label, count }) => ({
          key,
          label: (
            <span data-testid={`images-tab-${key}`} data-count={count}>
              {label} <Typography.Text type="secondary">{formatCount(count)}</Typography.Text>
            </span>
          ),
        }))}
      />
      <Space wrap size={token.marginSM} style={{ marginBottom: token.margin }}>
        <Input
          allowClear
          data-testid="images-search-input"
          prefix={<SearchOutlined />}
          placeholder={t('images.searchPlaceholder')}
          maxLength={200}
          value={text}
          onChange={(event) => onSearch(event.target.value)}
          style={{ width: 280, maxWidth: '100%' }}
        />
        <Select<ImageOrder>
          data-testid="images-sort-select"
          value={searching ? 'path' : filters.sort}
          disabled={searching}
          onChange={(sort) => sort !== 'path' && setFilters({ sort })}
          options={[
            { value: 'updated', label: t('images.sort.updated') },
            { value: 'saved', label: t('images.sort.saved'), disabled: filters.status !== 'optimized' },
            ...(searching ? [{ value: 'path', label: t('images.sort.path') }] : []),
          ]}
          style={{ minWidth: 180 }}
        />
      </Space>
      <ResponsiveTable
        testId="images-table"
        items={items}
        loading={images.isFetching && !images.isFetchingNextPage}
        columns={columns}
        emptyText={t('images.empty')}
        pagination={false}
        cardTestId={(image) => `image-card-${image.id}`}
        cardTitle={(image) => <ImagePath path={image.path} />}
        cardExtra={statusTag}
        renderCard={(image) => (
          <Space direction="vertical" size={token.marginXXS} style={{ width: '100%' }}>
            <Typography.Text type="secondary">
              {image.format.toUpperCase()} · <ImageSize image={image} /> · {updated(image)}
            </Typography.Text>
            <ImageReason image={image} />
            {retryButton(image)}
          </Space>
        )}
      />
      {images.hasNextPage && (
        <div style={{ display: 'flex', justifyContent: 'center', marginTop: token.margin }}>
          <Button data-testid="images-load-more-button" loading={images.isFetchingNextPage} onClick={() => images.fetchNextPage()}>
            {t('images.loadMore')}
          </Button>
        </div>
      )}
    </Card>
  )
}

/** The directory gives way first, so the file name stays readable; the tooltip has all of it. */
function ImagePath({ path, maxWidth }: { path: string; maxWidth?: number }) {
  const { token } = theme.useToken()
  const name = path.lastIndexOf('/') + 1
  const part = { overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 0 } as const
  return (
    <Tooltip title={path}>
      <span style={{ display: 'flex', maxWidth: maxWidth ?? '100%', whiteSpace: 'nowrap', fontFamily: token.fontFamilyCode }}>
        <span style={{ ...part, flex: '0 100 auto' }}>{path.slice(0, name)}</span>
        <span style={{ ...part, flex: '0 1 auto' }}>{path.slice(name)}</span>
      </span>
    </Tooltip>
  )
}

function ImageSize({ image }: { image: Image }) {
  const { before, after, savedPercent } = imageSizes(image)
  if (after === null) return <span data-testid={`image-size-${image.id}`}>{formatBytes(before)}</span>
  return (
    <span data-testid={`image-size-${image.id}`} data-before={before} data-after={after} style={{ whiteSpace: 'nowrap' }}>
      {formatBytes(before)} → {formatBytes(after)}
      {savedPercent !== null && <Typography.Text type="secondary"> −{formatNumber(savedPercent, 0)}%</Typography.Text>}
    </span>
  )
}

function ImageReason({ image, maxWidth }: { image: Image; maxWidth?: number }) {
  const { t } = useTranslation()
  const { reason, lossless, retry } = explainImage(t, image)
  return (
    <div
      data-testid={`image-reason-${image.id}`}
      data-reason-code={reason?.code ?? undefined}
      data-lossless-code={lossless?.code ?? undefined}
      style={{ maxWidth, whiteSpace: 'normal' }}
    >
      {!reason && !lossless && !retry && EMPTY}
      {reason && <ReasonLine reason={reason} />}
      {lossless && <ReasonLine reason={lossless} />}
      {retry && (
        <Typography.Text type="secondary" style={{ display: 'block' }}>
          {retry}
        </Typography.Text>
      )}
    </div>
  )
}

function ReasonLine({ reason }: { reason: ReasonText }) {
  const { token } = theme.useToken()
  return (
    <div>
      <Typography.Text>{reason.text}</Typography.Text>
      {reason.detail && (
        <Typography.Text type="secondary" style={{ display: 'block', fontSize: token.fontSizeSM, overflowWrap: 'anywhere' }}>
          {reason.detail}
        </Typography.Text>
      )}
    </div>
  )
}
