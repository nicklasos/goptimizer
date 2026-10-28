import { useCallback } from 'react'
import { useNavigate, type NavigateOptions, type To } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { confirmUnsavedNavigation } from './confirmUnsavedNavigation'
import { useUnsavedChangesContext } from './useUnsavedChangesContext'

export function useGuardedNavigate() {
  const navigate = useNavigate()
  const { isDirty } = useUnsavedChangesContext()
  const { t } = useTranslation()

  return useCallback(
    (to: To, options?: NavigateOptions) => {
      confirmUnsavedNavigation(isDirty, t, () => navigate(to, options))
    },
    [navigate, isDirty, t]
  )
}
