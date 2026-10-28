import { Space, Tag } from 'antd'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { ROLE_SUPER_ADMIN } from './roles'

/** A user's roles as quiet tags; super-admin is the one highlighted role. */
export function RoleTags({ roles }: { roles: string[] | null | undefined }) {
  const { t } = useTranslation()
  return (
    <Space size={4} wrap>
      {(roles ?? []).map((role) => (
        <Tag key={role} bordered={false} color={role === ROLE_SUPER_ADMIN ? 'processing' : undefined}>
          {enumLabel(t, 'role', role)}
        </Tag>
      ))}
    </Space>
  )
}
