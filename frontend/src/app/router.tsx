import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AuthCard } from '@/auth/AuthCard'
import { RequireRole } from '@/auth/guards'
import LoginPage from '@/auth/LoginPage'
import { ROLE_ADMIN } from '@/auth/roles'
import { AppShell } from '@/layout/AppShell'

// Every page is its own chunk, loaded when its route is first opened. The login form is
// the exception: it is the first thing a signed-out visitor sees.
const ForgotPasswordPage = lazy(() => import('@/auth/ForgotPasswordPage'))
const ResetPasswordPage = lazy(() => import('@/auth/ResetPasswordPage'))
const VerifyEmailPage = lazy(() => import('@/auth/VerifyEmailPage'))
const PlanEditorPage = lazy(() => import('@/features/plans/pages/PlanEditorPage'))
const PlanPage = lazy(() => import('@/features/plans/pages/PlanPage'))
const ProfilePage = lazy(() => import('@/features/account/ProfilePage'))
const DashboardPage = lazy(() => import('@/features/plans/pages/DashboardPage'))
const StorageBrowserPage = lazy(() => import('@/features/storages/pages/StorageBrowserPage'))
const StorageEditorPage = lazy(() => import('@/features/storages/pages/StorageEditorPage'))
const StoragesPage = lazy(() => import('@/features/storages/pages/StoragesPage'))
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
        <Route index element={<DashboardPage />} />
        <Route path="profile" element={<ProfilePage />} />
        <Route path="verify-email" element={<VerifyEmailPage />} />
        {/* The dashboard is the list of plans */}
        <Route path="plans" element={<Navigate to="/" replace />} />
        <Route
          path="plans/new"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <PlanEditorPage />
            </RequireRole>
          }
        />
        <Route
          path="plans/:id"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <PlanPage />
            </RequireRole>
          }
        />
        <Route
          path="plans/:id/edit"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <PlanEditorPage />
            </RequireRole>
          }
        />
        <Route
          path="storages"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <StoragesPage />
            </RequireRole>
          }
        />
        <Route
          path="storages/new"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <StorageEditorPage />
            </RequireRole>
          }
        />
        <Route
          path="storages/:id/edit"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <StorageEditorPage />
            </RequireRole>
          }
        />
        <Route
          path="storages/:id/browse"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <StorageBrowserPage />
            </RequireRole>
          }
        />
        <Route
          path="users"
          element={
            <RequireRole roles={[ROLE_ADMIN]}>
              <UsersPage />
            </RequireRole>
          }
        />
        {/* The admin and super admin lists used to have pages of their own */}
        <Route path="admin/super-admins" element={<Navigate to="/users?role=super-admin" replace />} />
        <Route path="admin/admins" element={<Navigate to="/users?role=admin" replace />} />
        {/* Signed-out pages make no sense once signed in */}
        <Route path="forgot-password" element={<Navigate to="/" replace />} />
        <Route path="reset-password" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  )
}
