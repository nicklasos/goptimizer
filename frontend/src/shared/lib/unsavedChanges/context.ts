import { createContext } from 'react'

export interface UnsavedChangesContextValue {
  setDirty: (isDirty: boolean) => void
  isDirty: () => boolean
}

export const UnsavedChangesContext = createContext<UnsavedChangesContextValue | null>(null)
