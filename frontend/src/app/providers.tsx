import type { ReactNode } from 'react'
import { ConfigProvider } from 'antd'
import { QueryClientProvider } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import enUS from 'antd/locale/en_US'
import ukUA from 'antd/locale/uk_UA'
import { UnsavedChangesProvider } from '@/shared/lib/unsavedChanges'
import { queryClient } from './queryClient'
import { theme } from './theme'

export function Providers({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation()
  const locale = i18n.language?.startsWith('uk') ? ukUA : enUS

  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider locale={locale} theme={theme}>
        <UnsavedChangesProvider>{children}</UnsavedChangesProvider>
      </ConfigProvider>
    </QueryClientProvider>
  )
}
