import { Alert, Descriptions, Typography, theme } from 'antd'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import type { SourceTestResult as Result } from '../types'

/** The outcome of "Test connection": what failed, and the server and dump program versions found. */
export function SourceTestResult({ result }: { result: Result }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const versions = (result.server_version || result.tool) && (
    <Descriptions size="small" column={1} style={{ marginTop: result.error ? token.marginXS : 0 }}>
      {result.server_version && (
        <Descriptions.Item label={t('plans.test.server')}>
          <span data-testid="plan-test-server-version">{result.server_version}</span>
        </Descriptions.Item>
      )}
      {result.tool && (
        <Descriptions.Item label={result.tool.name}>
          <span data-testid="plan-test-tool-version" data-found={result.tool.found}>
            {result.tool.found ? result.tool.version : t('plans.test.notFound')}
          </span>
        </Descriptions.Item>
      )}
    </Descriptions>
  )

  if (result.ok) {
    return (
      <div data-testid="plan-test-ok">
        <Alert type="success" showIcon message={t('plans.test.ok')} description={versions} />
      </div>
    )
  }
  const step = result.step ?? 'connect'
  return (
    <div data-testid="plan-test-failed" data-step={step}>
      <Alert
        type="error"
        showIcon
        message={t('plans.test.failed', { step: enumLabel(t, 'sourceTestStep', step) })}
        description={
          <>
            <Typography.Text style={{ wordBreak: 'break-word' }}>{result.error}</Typography.Text>
            {versions}
          </>
        }
      />
    </div>
  )
}
