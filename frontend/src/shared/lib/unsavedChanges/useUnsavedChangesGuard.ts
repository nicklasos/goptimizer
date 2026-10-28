import { useEffect } from 'react'
import { useUnsavedChangesContext } from './useUnsavedChangesContext'

export function useUnsavedChangesGuard(isDirty: boolean): void {
  const { setDirty } = useUnsavedChangesContext()

  useEffect(() => {
    setDirty(isDirty)
    return () => setDirty(false)
  }, [isDirty, setDirty])

  useEffect(() => {
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!isDirty) return
      event.preventDefault()
    }

    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [isDirty])
}
