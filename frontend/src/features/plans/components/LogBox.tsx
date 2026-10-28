import { useEffect, useRef } from 'react'
import { Spin, Typography, theme } from 'antd'
import { useTranslation } from 'react-i18next'

/** A job's log, kept scrolled to its end as it grows, with a spinner while the job is active. */
export function LogBox({ log, active, testId }: { log: string; active: boolean; testId: string }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const ref = useRef<HTMLPreElement>(null)

  useEffect(() => {
    const box = ref.current
    if (box) box.scrollTop = box.scrollHeight
  }, [log])

  return (
    <div>
      <Typography.Title level={5}>
        {t('runs.log')} {active && <Spin size="small" style={{ marginInlineStart: token.marginXS }} />}
      </Typography.Title>
      <pre
        ref={ref}
        data-testid={testId}
        style={{
          margin: 0,
          maxHeight: 420,
          overflow: 'auto',
          padding: token.paddingSM,
          background: token.colorFillQuaternary,
          border: `1px solid ${token.colorBorderSecondary}`,
          borderRadius: token.borderRadius,
          fontFamily: token.fontFamilyCode,
          fontSize: token.fontSizeSM,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}
      >
        {log || t('runs.logEmpty')}
      </pre>
    </div>
  )
}
