import { List, Tag, Typography } from 'antd'
import { FolderOpenOutlined } from '@ant-design/icons'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { browseUrl } from '../source'
import type { Plan } from '../types'

/** Where the plan uploads, each with a way into the plan's folder there. */
export function StoragesTab({ plan }: { plan: Plan }) {
  const { t } = useTranslation()
  return (
    <List
      data-testid="plan-storages-list"
      bordered
      dataSource={plan.storages}
      renderItem={(storage) => (
        <List.Item
          data-testid={`plan-storage-${storage.id}`}
          actions={[
            <Link key="browse" to={browseUrl(storage.id, plan.folder)} data-testid={`plan-storage-browse-${storage.id}`}>
              <FolderOpenOutlined /> {t('plans.page.browseFolder')}
            </Link>,
          ]}
        >
          <List.Item.Meta
            title={storage.name}
            description={
              <>
                <Tag bordered={false}>{enumLabel(t, 'storageKind', storage.kind)}</Tag>
                <Typography.Text type="secondary" code>
                  {plan.folder}
                </Typography.Text>
              </>
            }
          />
        </List.Item>
      )}
    />
  )
}
