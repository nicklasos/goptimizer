import type { Folder, FolderRequest, Profile } from './types'

/** The form edits the request as is: its field names are the API's field paths, so server errors land under them. */
export type FolderFormValues = Required<Pick<FolderRequest, 'name' | 'path' | 'scan_interval_minutes' | 'full_scan_interval_hours'>> & {
  profile: Profile
}

/** Mirrors folders.DefaultProfile and the default intervals in the API; it only fills the new-folder form. */
export const NEW_FOLDER: FolderFormValues = {
  name: '',
  path: '',
  scan_interval_minutes: 60,
  full_scan_interval_hours: 24,
  profile: {
    mode: 'lossy',
    jpeg_max_quality: 82,
    png_quality_min: 65,
    png_quality_max: 80,
    gif_lossy: 30,
    min_saving_percent: 3,
    min_ssim: 0.98,
    min_age_minutes: 5,
    excludes: [],
  },
}

const PROFILE_FIELDS: readonly (keyof Profile)[] = Object.keys(NEW_FOLDER.profile) as (keyof Profile)[]

export const FOLDER_FORM_FIELDS: readonly string[] = [
  'name',
  'path',
  'kind',
  'scan_interval_minutes',
  'full_scan_interval_hours',
  ...PROFILE_FIELDS.map((field) => `profile.${field}`),
]

export function folderToForm(folder: Folder): FolderFormValues {
  return {
    name: folder.name,
    path: folder.path,
    scan_interval_minutes: folder.scan_interval_minutes,
    full_scan_interval_hours: folder.full_scan_interval_hours,
    profile: folder.profile,
  }
}

export function formToRequest(values: FolderFormValues): FolderRequest {
  return { ...values, kind: 'local', name: values.name.trim(), path: values.path.trim() }
}
