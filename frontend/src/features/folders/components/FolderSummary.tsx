import { Alert, Tag } from 'antd'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import type { Folder } from '../types'

export function FolderModeTag({ folder }: { folder: Folder }) {
  const { t } = useTranslation()
  return (
    <Tag data-testid={`folder-mode-tag-${folder.id}`} data-mode={folder.profile.mode} color={folder.profile.mode === 'lossy' ? 'blue' : 'green'}>
      {enumLabel(t, 'mode', folder.profile.mode)}
    </Tag>
  )
}

/** Images of a format whose program is missing wait until it is installed: say so, with the way to /settings. */
export function MissingToolsAlert({ folder }: { folder: Folder }) {
  const { t } = useTranslation()
  const missingTools = folder.missing_tools
  if (missingTools.length === 0) return null
  return (
    <Alert
      type="warning"
      showIcon
      data-testid={`folder-missing-tools-${folder.id}`}
      message={t('folders.optimize.missingTools', { count: missingTools.length, tools: missingTools.join(', ') })}
      action={
        <Link to="/settings" data-testid={`folder-missing-tools-link-${folder.id}`}>
          {t('folders.optimize.install')}
        </Link>
      }
    />
  )
}
