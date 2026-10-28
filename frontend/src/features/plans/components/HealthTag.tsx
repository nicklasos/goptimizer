import { Tag, theme } from 'antd'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { HEALTH } from '../health'
import type { Health } from '../types'

/** A plan's health; `data-health` carries it for tests. */
export function HealthTag({ health, testId }: { health: Health; testId: string }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const { tag, color, Icon, spin } = HEALTH[health]
  // A preset colours its own icon; the grey of a default tag would leave the icon black.
  const icon = tag === 'default' ? <Icon spin={spin} style={{ color: token[color] }} /> : <Icon spin={spin} />
  return (
    <Tag bordered={false} color={tag} icon={icon} data-testid={testId} data-health={health} style={{ marginInlineEnd: 0 }}>
      {enumLabel(t, 'health', health)}
    </Tag>
  )
}
