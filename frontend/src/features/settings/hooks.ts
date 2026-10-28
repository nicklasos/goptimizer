import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { qk } from '@/api/queryKeys'
import { settingsApi } from './api'
import type { SettingsRequest } from './types'

export function useSettings() {
  return useQuery({ queryKey: qk.settings, queryFn: settingsApi.get })
}

/** Field errors are shown in the form, so the global error toast is silenced. */
export function useSaveSettings() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (body: SettingsRequest) => settingsApi.update(body),
    onSuccess: (settings) => client.setQueryData(qk.settings, settings),
    meta: { silent: true },
  })
}

export function useTools() {
  return useQuery({ queryKey: qk.tools, queryFn: () => settingsApi.tools(false) })
}

/** Looks the programs up again, past the API's short cache. */
export function useRefreshTools() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: () => settingsApi.tools(true),
    onSuccess: (report) => client.setQueryData(qk.tools, report),
  })
}
