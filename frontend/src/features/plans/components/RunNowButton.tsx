import { Button } from 'antd'
import { CaretRightOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import message from '@/shared/utils/message'
import { useRunPlan } from '../hooks'
import type { Health } from '../types'

/** Queues a backup of the plan. Off while one is queued or running. */
export function RunNowButton({ plan, testId, size }: { plan: { id: number; health: Health }; testId: string; size?: 'small' }) {
  const { t } = useTranslation()
  const run = useRunPlan()
  const busy = plan.health === 'running'
  const onClick = () => run.mutate(plan.id, { onSuccess: () => message.success(t('plans.runQueued')) })
  return (
    <Button size={size} icon={<CaretRightOutlined />} onClick={onClick} loading={run.isPending} disabled={busy} data-testid={testId}>
      {busy ? t('plans.runBusy') : t('plans.runNow')}
    </Button>
  )
}
