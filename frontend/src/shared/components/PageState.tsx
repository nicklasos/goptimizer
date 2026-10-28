import { Button, Result, Spin } from 'antd'
import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { isApiError } from '@/api/errors'
import type { PageBack } from './PageHeader'

/** Centred loading state for a whole page. */
export function PageLoading() {
  return (
    <div data-testid="page-loading" style={{ display: 'flex', justifyContent: 'center', padding: '64px 0' }}>
      <Spin size="large" />
    </div>
  )
}

interface ErrorProps {
  error: unknown
  back?: PageBack
  onRetry?: () => void
}

/** "Not found" only for a real 404; any other failure says so and offers a retry. */
export function PageError({ error, back, onRetry }: ErrorProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const notFound = !error || (isApiError(error) && error.status === 404)
  const target = back ?? { label: t('common.back'), onClick: () => navigate(-1), testId: 'page-back-button' }
  const backButton = (
    <Button key="back" data-testid={target.testId} icon={<ArrowLeftOutlined />} onClick={target.onClick}>
      {target.label}
    </Button>
  )
  if (notFound) {
    return (
      <div data-testid="page-not-found">
        <Result status="404" title={t('common.notFound')} extra={backButton} />
      </div>
    )
  }
  return (
    <div data-testid="page-error">
      <Result
        status="error"
        title={t('errors.fallback')}
        extra={[
          onRetry && (
            <Button key="retry" type="primary" icon={<ReloadOutlined />} onClick={onRetry} data-testid="page-retry-button">
              {t('common.retry')}
            </Button>
          ),
          backButton,
        ]}
      />
    </div>
  )
}
