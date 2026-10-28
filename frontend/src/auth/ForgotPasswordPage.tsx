import { Alert, Button, Form, Input, Result, theme } from 'antd'
import { MailOutlined } from '@ant-design/icons'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { isApiError } from '@/api/errors'
import { errorMessage } from '@/shared/utils/serverErrors'
import { AuthCard } from './AuthCard'
import { useForgotPassword } from './hooks'
import { retryAfterMessage } from './retryAfter'

export default function ForgotPasswordPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const request = useForgotPassword()

  const backToLogin = (
    <div style={{ textAlign: 'center' }}>
      <Link to="/" data-testid="back-to-login-link">
        {t('auth.backToLogin')}
      </Link>
    </div>
  )

  if (request.isSuccess) {
    return (
      <AuthCard subtitle={t('auth.forgotPasswordTitle')}>
        <div data-testid="forgot-password-sent">
          <Result status="success" title={t('auth.resetLinkSentTitle')} subTitle={t('auth.resetLinkSent')} style={{ padding: 0, marginBottom: token.marginLG }} />
        </div>
        {backToLogin}
      </AuthCard>
    )
  }

  const describe = (error: unknown) =>
    isApiError(error) && error.status === 429
      ? retryAfterMessage(t, Number(error.details?.retry_after_seconds) || 60)
      : errorMessage(t, error, 'errors.network')

  return (
    <AuthCard subtitle={t('auth.forgotPasswordHint')}>
      {request.isError && (
        <Alert data-testid="forgot-password-error" type="error" showIcon message={describe(request.error)} style={{ marginBottom: token.marginMD }} />
      )}

      <Form layout="vertical" size="large" onFinish={(values: { email: string }) => request.mutate(values.email)} data-testid="forgot-password-form">
        <Form.Item
          name="email"
          label={t('auth.email')}
          rules={[
            { required: true, message: t('auth.emailRequired') },
            { type: 'email', message: t('auth.emailInvalid') },
          ]}
        >
          <Input data-testid="forgot-password-email-input" prefix={<MailOutlined />} placeholder="user@example.com" autoComplete="email" />
        </Form.Item>

        <Form.Item style={{ marginBottom: token.marginSM }}>
          <Button data-testid="forgot-password-submit-button" type="primary" htmlType="submit" loading={request.isPending} block>
            {t('auth.sendResetLink')}
          </Button>
        </Form.Item>
        {backToLogin}
      </Form>
    </AuthCard>
  )
}
