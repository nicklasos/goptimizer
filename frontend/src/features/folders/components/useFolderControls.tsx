import { Button, Dropdown, Space, Switch, Typography, theme } from 'antd'
import { DownOutlined, EditOutlined, SyncOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useSetFolderEnabled, useStartScan } from '../hooks'
import { isScanActive } from '../scans'
import { SCAN_KINDS, type Folder, type ScanKind } from '../types'

/**
 * A folder's pause switch, Scan now menu and Edit button, for its card and its page to place.
 * `enabled` is what the switch shows: the value being saved while it is, the folder's otherwise.
 */
export function useFolderControls(folder: Folder, buttonType: 'link' | 'default') {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const setEnabled = useSetFolderEnabled()
  const startScan = useStartScan()
  const pending = setEnabled.isPending ? setEnabled.variables : undefined
  const enabled = pending?.id === folder.id ? pending.enabled : folder.enabled
  const scanning = isScanActive(folder.last_scan)

  const enabledSwitch = (
    <Space key="enabled" size={token.marginXS}>
      <Typography.Text type="secondary">{enabled ? t('folders.active') : t('folders.paused')}</Typography.Text>
      <Switch
        data-testid={`folder-enabled-switch-${folder.id}`}
        checked={enabled}
        loading={setEnabled.isPending}
        onChange={(checked) => setEnabled.mutate({ id: folder.id, enabled: checked })}
      />
    </Space>
  )

  const scanButton = (
    <Dropdown
      key="scan"
      trigger={['click']}
      disabled={!enabled || scanning}
      menu={{
        items: SCAN_KINDS.map((kind) => ({
          key: kind,
          label: <span data-testid={`${kind}-scan-item-${folder.id}`}>{t(`folders.scan.${kind}`)}</span>,
        })),
        onClick: ({ key }) => startScan.mutate({ id: folder.id, kind: key as ScanKind }),
      }}
    >
      <Button
        type={buttonType}
        icon={<SyncOutlined spin={scanning} />}
        loading={startScan.isPending}
        disabled={!enabled || scanning}
        data-testid={`scan-folder-button-${folder.id}`}
      >
        {t('folders.scan.now')} <DownOutlined />
      </Button>
    </Dropdown>
  )

  const editButton = (
    <Button
      key="edit"
      type={buttonType}
      icon={<EditOutlined />}
      data-testid={`edit-folder-button-${folder.id}`}
      onClick={() => navigate(`/folders/${folder.id}/edit`)}
    >
      {t('common.edit')}
    </Button>
  )

  return { enabled, enabledSwitch, scanButton, editButton }
}
