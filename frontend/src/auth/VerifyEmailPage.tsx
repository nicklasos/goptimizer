import { useEffect, useRef } from 'react'
import { Button, Result, Spin } from 'antd'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { errorMessage } from '@/shared/utils/serverErrors'
import { useAuthStore } from './authStore'
import { useVerifyEmail } from './hooks'

/** Confirms the token from the emailed link. Works signed in or out; the link is single-use. */
export default function VerifyEmailPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [search] = useSearchParams()
  const token = search.get('token') ?? ''
  const { mutate, isSuccess, isError, error } = useVerifyEmail()
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const me = useAuthStore((s) => s.me)
  const submitted = useRef(false)

  useEffect(() => {
    if (!token || submitted.current) return
    submitted.current = true
    mutate(token, {
      onSuccess: () => {
        if (isAuthenticated) void me()
      },
    })
  }, [token, mutate, isAuthenticated, me])

  const done = (
    <Button type="primary" onClick={() => navigate('/', { replace: true })} data-testid="verify-email-continue-button">
      {t('auth.continue')}
    </Button>
  )

  if (isSuccess) {
    return (
      <div data-testid="verify-email-success">
        <Result status="success" title={t('auth.emailVerified')} extra={done} />
      </div>
    )
  }

  if (!token || isError) {
    return (
      <div data-testid="verify-email-error">
        <Result
          status="warning"
          title={token ? errorMessage(t, error, 'errors.network') : t('errors.auth.invalid_or_expired_link')}
          subTitle={t('auth.verifyEmailRetryHint')}
          extra={done}
        />
      </div>
    )
  }

  return (
    <div data-testid="verify-email-loading" style={{ display: 'flex', justifyContent: 'center', padding: '64px 0' }}>
      <Spin size="large" />
    </div>
  )
}
