import { useState } from 'react'
import { Alert, Button, Form, Input, Result, theme } from 'antd'
import { LockOutlined } from '@ant-design/icons'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import message from '@/shared/utils/message'
import { applyFormErrors } from '@/shared/utils/formErrors'
import { AuthCard } from './AuthCard'
import { useResetPassword } from './hooks'

interface Values {
  password: string
  confirm: string
}

export default function ResetPasswordPage() {
  const { t } = useTranslation()
  const { token: themeToken } = theme.useToken()
  const navigate = useNavigate()
  const [search] = useSearchParams()
  const [form] = Form.useForm<Values>()
  const reset = useResetPassword()
  const [serverError, setServerError] = useState<string | null>(null)
  const token = search.get('token') ?? ''

  const requestNewLink = (
    <div style={{ textAlign: 'center' }}>
      <Link to="/forgot-password" data-testid="request-new-link">
        {t('auth.requestNewLink')}
      </Link>
    </div>
  )

  if (!token) {
    return (
      <AuthCard subtitle={t('auth.resetPasswordTitle')}>
        <div data-testid="reset-password-invalid">
          <Result status="warning" title={t('errors.auth.invalid_or_expired_link')} style={{ padding: 0, marginBottom: themeToken.marginLG }} />
        </div>
        {requestNewLink}
      </AuthCard>
    )
  }

  const onFinish = (values: Values) => {
    setServerError(null)
    reset.mutate(
      { token, password: values.password },
      {
        onSuccess: () => {
          message.success(t('auth.passwordResetDone'))
          navigate('/', { replace: true })
        },
        onError: (error) => setServerError(applyFormErrors(form, t, error, ['password'])),
      }
    )
  }

  return (
    <AuthCard subtitle={t('auth.resetPasswordTitle')}>
      {serverError && (
        <Alert data-testid="reset-password-error" type="error" showIcon message={serverError} style={{ marginBottom: themeToken.marginMD }} />
      )}

      <Form form={form} layout="vertical" size="large" onFinish={onFinish} data-testid="reset-password-form">
        <Form.Item
          name="password"
          label={t('profile.newPassword')}
          rules={[
            { required: true, message: t('common.required') },
            { min: 8, message: t('userManagement.passwordTooShort') },
          ]}
        >
          <Input.Password data-testid="reset-password-input" prefix={<LockOutlined />} autoComplete="new-password" />
        </Form.Item>

        <Form.Item
          name="confirm"
          label={t('profile.confirmPassword')}
          dependencies={['password']}
          rules={[
            { required: true, message: t('common.required') },
            ({ getFieldValue }) => ({
              validator: (_, value) =>
                !value || getFieldValue('password') === value ? Promise.resolve() : Promise.reject(new Error(t('profile.passwordMismatch'))),
            }),
          ]}
        >
          <Input.Password data-testid="reset-password-confirm-input" prefix={<LockOutlined />} autoComplete="new-password" />
        </Form.Item>

        <Form.Item style={{ marginBottom: themeToken.marginSM }}>
          <Button data-testid="reset-password-submit-button" type="primary" htmlType="submit" loading={reset.isPending} block>
            {t('auth.setNewPassword')}
          </Button>
        </Form.Item>
        {reset.isError && requestNewLink}
      </Form>
    </AuthCard>
  )
}
