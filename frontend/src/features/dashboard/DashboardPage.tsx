import { Card, Empty } from 'antd'
import { DashboardOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageStack } from '@/shared/components/PageStack'

export default function DashboardPage() {
  const { t } = useTranslation()
  return (
    <PageStack testId="dashboard-page">
      <Card>
        <PageHeader icon={<DashboardOutlined />} title={t('dashboard.title')} />
        <Empty data-testid="dashboard-empty" description={t('dashboard.empty')} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </Card>
    </PageStack>
  )
}
