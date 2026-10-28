import { Result } from 'antd'
import { useTranslation } from 'react-i18next'

export function Forbidden() {
  const { t } = useTranslation()
  return (
    <div data-testid="forbidden-result">
      <Result status="403" title={t('auth.accessDenied')} subTitle={t('auth.noPermission')} />
    </div>
  )
}
