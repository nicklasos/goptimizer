import { Tag } from 'antd'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { RUN_STATUS_COLOR } from '../runStatus'
import type { RunStatus } from '../types'

/** A run's status; `data-status` carries it for tests. */
export function RunStatusTag({ status, testId }: { status: RunStatus; testId: string }) {
  const { t } = useTranslation()
  return (
    <Tag bordered={false} color={RUN_STATUS_COLOR[status]} data-testid={testId} data-status={status}>
      {enumLabel(t, 'runStatus', status)}
    </Tag>
  )
}
