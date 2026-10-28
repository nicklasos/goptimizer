import { Button, Result } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'

interface Props {
  /** Clears the error and renders the page again. */
  onRetry: () => void
  /** Fills the window, for a crash outside the app shell. */
  fullScreen?: boolean
}

/** What the user sees when a page throws while rendering. */
export function CrashScreen({ onRetry, fullScreen }: Props) {
  const { t } = useTranslation()
  return (
    <div
      data-testid="crash-screen"
      style={fullScreen ? { minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' } : undefined}
    >
      <Result
        status="error"
        title={t('errors.crashTitle')}
        subTitle={t('errors.crashHint')}
        extra={[
          <Button key="retry" type="primary" onClick={onRetry} data-testid="crash-retry-button">
            {t('common.retry')}
          </Button>,
          <Button key="reload" icon={<ReloadOutlined />} onClick={() => window.location.reload()} data-testid="crash-reload-button">
            {t('errors.reloadPage')}
          </Button>,
        ]}
      />
    </div>
  )
}
