import type {
  AppInternalToolsNeedStatus,
  AppInternalToolsProgramStatus,
  AppInternalToolsReport,
  InternalSettingsSettingsRequest,
  InternalSettingsSettingsResponse,
} from '@/api/schema'

export type Settings = InternalSettingsSettingsResponse
export type SettingsRequest = InternalSettingsSettingsRequest
export type ToolsReport = AppInternalToolsReport
export type ToolProgram = AppInternalToolsProgramStatus
export type NeedStatus = AppInternalToolsNeedStatus
