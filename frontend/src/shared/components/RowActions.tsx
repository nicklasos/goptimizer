import type { ReactNode } from 'react'
import { Button, Popconfirm, Space, Tooltip } from 'antd'
import { DeleteOutlined, EditOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'

interface Props {
  entity: string
  id: number
  onEdit?: () => void
  onDelete?: () => void
  deleteTitle?: string
  deleteDescription?: string
  children?: ReactNode
}

export function RowActions({ entity, id, onEdit, onDelete, deleteTitle, deleteDescription, children }: Props) {
  const { t } = useTranslation()
  return (
    <Space size="small">
      {onEdit && (
        <Tooltip title={t('common.edit')}>
          <Button type="text" icon={<EditOutlined />} onClick={onEdit} size="small" data-testid={`edit-${entity}-button-${id}`} />
        </Tooltip>
      )}
      {children}
      {onDelete && (
        <Popconfirm
          title={deleteTitle}
          description={deleteDescription}
          onConfirm={onDelete}
          okText={t('common.yes')}
          cancelText={t('common.no')}
          okButtonProps={{ danger: true, 'data-testid': `confirm-delete-${entity}-button-${id}` }}
        >
          <Tooltip title={t('common.delete')}>
            <Button type="text" icon={<DeleteOutlined />} danger size="small" data-testid={`delete-${entity}-button-${id}`} />
          </Tooltip>
        </Popconfirm>
      )}
    </Space>
  )
}
