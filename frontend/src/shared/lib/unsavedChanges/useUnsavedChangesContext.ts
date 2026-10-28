import { useContext } from 'react'
import { UnsavedChangesContext, type UnsavedChangesContextValue } from './context'

export function useUnsavedChangesContext(): UnsavedChangesContextValue {
  const context = useContext(UnsavedChangesContext)
  if (!context) {
    throw new Error('useUnsavedChangesContext must be used within UnsavedChangesProvider')
  }
  return context
}
