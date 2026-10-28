import { useEffect } from 'react'
import { Alert, Button, Form, Input, theme } from 'antd'
import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { translateServerError } from '@/shared/utils/serverErrors'
import { AuthCard } from './AuthCard'
import { useAuthStore, type AuthError } from './authStore'
import { retryAfterMessage } from './retryAfter'

export default function LoginPage() {
  const [form] = Form.useForm()
  const { t } = useTranslation()
  const { login, loading, error, clearError } = useAuthStore()
  const { token } = theme.useToken()

  useEffect(() => {
    clearError()
  }, [clearError])

  const describe = (err: AuthError): string => {
    if (err.type === 'rate_limit') return retryAfterMessage(t, err.retryAfterSeconds)
    if (err.type === 'raw') return t('errors.network')
    return translateServerError(t, err.errorKey, err.message || t('auth.invalidCredentials'))
  }

  return (
    <AuthCard subtitle={t('auth.welcome')}>
      {error && (
        <Alert
          data-testid="login-error-alert"
          data-error-type={error.type}
          message={describe(error)}
          type={error.type === 'rate_limit' ? 'warning' : 'error'}
          showIcon
          style={{ marginBottom: token.marginMD }}
          closable
          onClose={clearError}
        />
      )}

      <Form
        form={form}
        name="login"
        onFinish={(values: { email: string; password: string }) => login(values.email, values.password)}
        layout="vertical"
        size="large"
        data-testid="login-form"
      >
        <Form.Item
          name="email"
          label={t('auth.email')}
          rules={[
            { required: true, message: t('auth.emailRequired') },
            { type: 'email', message: t('auth.emailInvalid') },
          ]}
        >
          <Input data-testid="login-email-input" prefix={<UserOutlined />} placeholder="user@example.com" autoComplete="email" />
        </Form.Item>

        <Form.Item name="password" label={t('auth.password')} rules={[{ required: true, message: t('auth.passwordRequired') }]}>
          <Input.Password
            data-testid="login-password-input"
            prefix={<LockOutlined />}
            placeholder={t('auth.password')}
            autoComplete="current-password"
          />
        </Form.Item>

        <Form.Item style={{ marginBottom: token.marginSM }}>
          <Button data-testid="login-button" type="primary" htmlType="submit" loading={loading} block>
            {t('auth.loginButton')}
          </Button>
        </Form.Item>

        <div style={{ textAlign: 'center' }}>
          <Link to="/forgot-password" data-testid="forgot-password-link">
            {t('auth.forgotPassword')}
          </Link>
        </div>
      </Form>
    </AuthCard>
  )
}
