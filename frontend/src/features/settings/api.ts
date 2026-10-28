import { api } from '@/api/client'
import type { Settings, SettingsRequest, ToolsReport } from './types'

export const settingsApi = {
  get: () => api.get<Settings>('/settings'),
  update: (body: SettingsRequest) => api.put<Settings>('/settings', body),
  tools: (refresh: boolean) => api.get<ToolsReport>('/tools', { query: refresh ? { refresh: 1 } : undefined }),
}
