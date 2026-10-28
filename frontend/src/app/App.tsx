import { Suspense, useEffect } from 'react'
import { BrowserRouter } from 'react-router-dom'
import { useAuthStore } from '@/auth/authStore'
import { CrashScreen } from '@/shared/components/CrashScreen'
import { ErrorBoundary } from '@/shared/components/ErrorBoundary'
import { PageLoading } from '@/shared/components/PageState'
import { Providers } from './providers'
import { queryClient } from './queryClient'
import { AppRoutes, PublicRoutes } from './router'

export default function App() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const me = useAuthStore((s) => s.me)

  useEffect(() => {
    if (isAuthenticated) me()
    else queryClient.clear()
  }, [isAuthenticated, me])

  return (
    <BrowserRouter>
      <Providers>
        {/* Last line of defence: a crash in the shell or on a signed-out page. Pages inside the shell have their own boundary. */}
        <ErrorBoundary fallback={(_, reset) => <CrashScreen onRetry={reset} fullScreen />}>
          <Suspense fallback={<PageLoading />}>{isAuthenticated ? <AppRoutes /> : <PublicRoutes />}</Suspense>
        </ErrorBoundary>
      </Providers>
    </BrowserRouter>
  )
}
