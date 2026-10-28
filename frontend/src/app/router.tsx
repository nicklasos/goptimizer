import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AuthCard } from '@/auth/AuthCard'
import { RequireAdmin } from '@/auth/guards'
import LoginPage from '@/auth/LoginPage'
import { AppShell } from '@/layout/AppShell'

// Every page is its own chunk, loaded when its route is first opened. The login form is
// the exception: it is the first thing a signed-out visitor sees.
const ForgotPasswordPage = lazy(() => import('@/auth/ForgotPasswordPage'))
const ResetPasswordPage = lazy(() => import('@/auth/ResetPasswordPage'))
const VerifyEmailPage = lazy(() => import('@/auth/VerifyEmailPage'))
const ProfilePage = lazy(() => import('@/features/account/ProfilePage'))
const FoldersPage = lazy(() => import('@/features/folders/pages/FoldersPage'))
const FolderEditorPage = lazy(() => import('@/features/folders/pages/FolderEditorPage'))
const SettingsPage = lazy(() => import('@/features/settings/pages/SettingsPage'))
const UsersPage = lazy(() => import('@/features/users/pages/UsersPage'))
const NotFoundPage = lazy(() => import('@/shared/components/NotFoundPage'))

/** Signed-out routes. Every other URL shows the login form in place, so signing in lands on the page that was asked for. */
export function PublicRoutes() {
  return (
    <Routes>
      <Route path="forgot-password" element={<ForgotPasswordPage />} />
      <Route path="reset-password" element={<ResetPasswordPage />} />
      <Route
        path="verify-email"
        element={
          <AuthCard subtitle={null}>
            <VerifyEmailPage />
          </AuthCard>
        }
      />
      <Route path="*" element={<LoginPage />} />
    </Routes>
  )
}

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<FoldersPage />} />
        <Route path="folders/new" element={<FolderEditorPage />} />
        <Route path="folders/:id/edit" element={<FolderEditorPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="profile" element={<ProfilePage />} />
        <Route path="verify-email" element={<VerifyEmailPage />} />
        <Route
          path="users"
          element={
            <RequireAdmin>
              <UsersPage />
            </RequireAdmin>
          }
        />
        {/* Signed-out pages make no sense once signed in */}
        <Route path="forgot-password" element={<Navigate to="/" replace />} />
        <Route path="reset-password" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  )
}
