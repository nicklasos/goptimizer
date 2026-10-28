import { Checkbox, Form, InputNumber, Space, Switch, Typography, theme } from 'antd'
import { useTranslation } from 'react-i18next'
import { formRetention, retentionSummary, type PlanFormValues } from '../planForm'

type RetentionValues = Pick<PlanFormValues, 'prune' | 'max_age_days' | 'keep_weekly_on' | 'keep_weekly' | 'keep_monthly_on' | 'keep_monthly'>

interface Props {
  values: RetentionValues
}

/** The "Keep" section: an age after which archives go, and weekly and monthly ones kept beyond it. */
export function RetentionFields({ values }: Props) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const row = { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: token.marginXS } as const

  return (
    <>
      <label style={{ ...row, marginBottom: token.marginSM, cursor: 'pointer' }}>
        <Form.Item name="prune" valuePropName="checked" noStyle>
          <Switch data-testid="plan-prune-switch" />
        </Form.Item>
        <Typography.Text>{t('plans.keep.prune')}</Typography.Text>
      </label>

      {values.prune && (
        <Space direction="vertical" size={token.marginSM} style={{ width: '100%', marginBottom: token.marginSM }}>
          <div style={row}>
            <Typography.Text>{t('plans.keep.olderThan')}</Typography.Text>
            <Form.Item name="max_age_days" noStyle rules={[{ required: true, message: t('common.required') }]}>
              <InputNumber data-testid="plan-max-age-input" min={1} max={3650} precision={0} style={{ width: 96 }} />
            </Form.Item>
            <Typography.Text>{t('plans.keep.days')}</Typography.Text>
          </div>
          <KeepRow
            on="keep_weekly_on"
            count="keep_weekly"
            max={520}
            enabled={values.keep_weekly_on}
            label={t('plans.keep.weekly')}
            unit={t('plans.keep.weeks')}
            testId="plan-keep-weekly"
          />
          <KeepRow
            on="keep_monthly_on"
            count="keep_monthly"
            max={240}
            enabled={values.keep_monthly_on}
            label={t('plans.keep.monthly')}
            unit={t('plans.keep.months')}
            testId="plan-keep-monthly"
          />
        </Space>
      )}

      <Typography.Paragraph type="secondary" data-testid="plan-retention-summary" style={{ marginBottom: 0 }}>
        {retentionSummary(t, formRetention(values))}
      </Typography.Paragraph>
    </>
  )
}

interface KeepRowProps {
  on: 'keep_weekly_on' | 'keep_monthly_on'
  count: 'keep_weekly' | 'keep_monthly'
  max: number
  enabled: boolean
  label: string
  unit: string
  testId: string
}

function KeepRow({ on, count, max, enabled, label, unit, testId }: KeepRowProps) {
  const { token } = theme.useToken()
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: token.marginXS }}>
      <Form.Item name={on} valuePropName="checked" noStyle>
        <Checkbox data-testid={`${testId}-checkbox`}>{label}</Checkbox>
      </Form.Item>
      <Form.Item name={count} noStyle>
        <InputNumber data-testid={`${testId}-input`} min={1} max={max} precision={0} disabled={!enabled} style={{ width: 96 }} />
      </Form.Item>
      <Typography.Text type={enabled ? undefined : 'secondary'}>{unit}</Typography.Text>
    </div>
  )
}
