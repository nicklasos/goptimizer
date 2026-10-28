import { useCallback, useMemo, useRef, type ReactNode } from 'react'
import { UnsavedChangesContext } from './context'

export function UnsavedChangesProvider({ children }: { children: ReactNode }) {
  const dirtyRef = useRef(false)

  const setDirty = useCallback((isDirty: boolean) => {
    dirtyRef.current = Boolean(isDirty)
  }, [])

  const isDirty = useCallback(() => dirtyRef.current, [])

  const value = useMemo(() => ({ setDirty, isDirty }), [setDirty, isDirty])

  return <UnsavedChangesContext.Provider value={value}>{children}</UnsavedChangesContext.Provider>
}
