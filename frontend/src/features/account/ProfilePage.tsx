import { useEffect } from 'react'
import { Alert, Card, Form, Input, Button, Segmented } from 'antd'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useCurrentUser } from '@/auth/authStore'
import { LANGUAGES } from '@/shared/utils/languages'
import { PageHeader } from '@/shared/components/PageHeader'
import message from '@/shared/utils/message'
import { useChangePassword, useResendVerification, useUpdateProfile } from './hooks'
import { PageStack } from '@/shared/components/PageStack'

interface PasswordValues {
  currentPassword: string
  newPassword: string
  confirmPassword: string
}

export default function ProfilePage() {
  const { t, i18n } = useTranslation()
  const user = useCurrentUser()
  const updateProfile = useUpdateProfile()
  const changePassword = useChangePassword()
  const resend = useResendVerification()

  const [profileForm] = Form.useForm<{ name: string; email: string }>()
  const [passwordForm] = Form.useForm<PasswordValues>()

  useEffect(() => {
    if (user) profileForm.setFieldsValue({ name: user.name, email: user.email })
  }, [user, profileForm])

  const handleProfileUpdate = (values: { name: string; email: string }) =>
    updateProfile.mutate(values, { onSuccess: () => message.success(t('profile.profileUpdated')) })

  const handlePasswordUpdate = (values: PasswordValues) =>
    changePassword.mutate(
      { current_password: values.currentPassword, new_password: values.newPassword },
      {
        onSuccess: () => {
          message.success(t('profile.passwordChanged'))
          passwordForm.resetFields()
        },
      }
    )

  return (
    <PageStack testId="profile-page" style={{ maxWidth: 640, margin: '0 auto', width: '100%' }}>
      <Card>
        <PageHeader icon={<UserOutlined />} title={t('profile.title')} subtitle={user?.email} />
      </Card>

      {user && !user.email_verified && (
        <Alert
          data-testid="email-unverified-alert"
          type="warning"
          showIcon
          message={t('profile.emailNotVerified')}
          description={t('profile.emailNotVerifiedHint', { email: user.email })}
          action={
            <Button
              data-testid="resend-verification-button"
              size="small"
              loading={resend.isPending}
              onClick={() => resend.mutate(undefined, { onSuccess: () => message.success(t('profile.verificationSent')) })}
            >
              {t('profile.resendVerification')}
            </Button>
          }
        />
      )}

      <Card data-testid="profile-info-card" title={t('profile.personalInfo')}>
        <Form form={profileForm} layout="vertical" onFinish={handleProfileUpdate} data-testid="profile-form">
          <Form.Item
            name="email"
            label={t('auth.email')}
            rules={[
              { required: true, message: t('common.required') },
              { type: 'email', message: t('auth.emailInvalid') },
            ]}
          >
            <Input data-testid="profile-email-input" prefix={<UserOutlined />} />
          </Form.Item>

          <Form.Item
            name="name"
            label={t('profile.name')}
            rules={[
              { required: true, message: t('common.required') },
              { max: 255, message: t('errors.validation.max') },
            ]}
          >
            <Input data-testid="profile-name-input" prefix={<UserOutlined />} />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button data-testid="profile-update-button" type="primary" htmlType="submit" loading={updateProfile.isPending}>
              {t('profile.updateProfile')}
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card data-testid="profile-language-card" title={t('profile.language')}>
        <Segmented
          data-testid="profile-language-picker"
          value={i18n.resolvedLanguage}
          onChange={(lng) => void i18n.changeLanguage(lng)}
          options={LANGUAGES.map(({ code, name }) => ({
            value: code,
            label: <span data-testid={`language-option-${code}`} lang={code}>{name}</span>,
          }))}
        />
      </Card>

      <Card data-testid="profile-password-card" title={t('profile.changePassword')}>
        <Form form={passwordForm} layout="vertical" onFinish={handlePasswordUpdate} data-testid="password-form">
          <Form.Item
            name="currentPassword"
            label={t('profile.currentPassword')}
            rules={[{ required: true, message: t('common.required') }]}
          >
            <Input.Password data-testid="profile-current-password-input" prefix={<LockOutlined />} />
          </Form.Item>

          <Form.Item
            name="newPassword"
            label={t('profile.newPassword')}
            rules={[
              { required: true, message: t('common.required') },
              { min: 8, message: t('userManagement.passwordTooShort') },
            ]}
          >
            <Input.Password data-testid="profile-new-password-input" prefix={<LockOutlined />} />
          </Form.Item>

          <Form.Item
            name="confirmPassword"
            label={t('profile.confirmPassword')}
            dependencies={['newPassword']}
            rules={[
              { required: true, message: t('common.required') },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve()
                  }
                  return Promise.reject(new Error(t('profile.passwordMismatch')))
                },
              }),
            ]}
          >
            <Input.Password data-testid="profile-confirm-password-input" prefix={<LockOutlined />} />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button data-testid="profile-change-password-button" type="primary" htmlType="submit" loading={changePassword.isPending} danger>
              {t('profile.changePassword')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </PageStack>
  )
}
