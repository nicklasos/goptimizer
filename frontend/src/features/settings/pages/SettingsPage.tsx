import { useEffect } from 'react'
import { Alert, Button, Card, Form, InputNumber, Space, Switch, Tag, Typography, theme } from 'antd'
import { ReloadOutlined, SettingOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageStack } from '@/shared/components/PageStack'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { EMPTY } from '@/shared/utils/format'
import { enumLabel } from '@/shared/utils/enums'
import { applyFormErrors } from '@/shared/utils/formErrors'
import message from '@/shared/utils/message'
import { useRefreshTools, useSaveSettings, useSettings, useTools } from '../hooks'
import { installCommands } from '../install'
import type { SettingsRequest, ToolProgram } from '../types'

type Row = ToolProgram & { id: number }

export default function SettingsPage() {
  return (
    <PageStack testId="settings-page">
      <ToolsCard />
      <SettingsCard />
    </PageStack>
  )
}

function ToolsCard() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const tools = useTools()
  const refresh = useRefreshTools()
  const rows: Row[] | undefined = tools.data?.programs.map((program, index) => ({ ...program, id: index }))
  const unmet = tools.data?.needs.filter((need) => !need.satisfied) ?? []

  const purpose = (program: ToolProgram) => t(`settings.purpose.${program.name}`, { defaultValue: program.purpose })
  const usedFor = (program: ToolProgram) => (
    <Space size={[0, token.marginXXS]} wrap>
      {program.used_for.map((need) => (
        <Tag key={need}>{enumLabel(t, 'need', need)}</Tag>
      ))}
    </Space>
  )
  const status = (program: ToolProgram) => (
    <Tag data-testid={`tool-status-${program.name}`} data-installed={program.installed} color={program.installed ? 'success' : 'error'}>
      {program.installed ? t('settings.installed') : t('settings.missing')}
    </Tag>
  )
  const requirement = (program: ToolProgram) =>
    program.alternatives.length ? (
      <Typography.Text type="secondary">{t('settings.orAlternative', { names: program.alternatives.join(', ') })}</Typography.Text>
    ) : (
      <Typography.Text type="secondary">{t('settings.required')}</Typography.Text>
    )
  const location = (program: ToolProgram) =>
    program.installed ? (
      <Space direction="vertical" size={0} style={{ maxWidth: 240 }}>
        <Typography.Text ellipsis={{ tooltip: program.version }}>{program.version || EMPTY}</Typography.Text>
        <Typography.Text type="secondary" ellipsis={{ tooltip: program.path }} style={{ fontSize: token.fontSizeSM }}>
          {program.path}
        </Typography.Text>
      </Space>
    ) : (
      EMPTY
    )
  const install = (program: ToolProgram) => {
    const commands = installCommands(program)
    return (
      <Space direction="vertical" size={token.marginXXS} style={{ maxWidth: 240 }}>
        {commands.apt ? (
          <Typography.Text code copyable data-testid={`tool-apt-${program.name}`}>
            {commands.apt}
          </Typography.Text>
        ) : (
          <Typography.Text type="secondary">{t('settings.noApt', { names: program.alternatives.join(', ') })}</Typography.Text>
        )}
        <Typography.Text code copyable data-testid={`tool-brew-${program.name}`}>
          {commands.brew}
        </Typography.Text>
      </Space>
    )
  }

  const columns: ColumnsType<Row> = [
    {
      title: t('settings.program'),
      key: 'name',
      width: 320,
      render: (_, program) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{program.name}</Typography.Text>
          <span>{purpose(program)}</span>
          {requirement(program)}
        </Space>
      ),
    },
    { title: t('settings.usedFor'), key: 'used_for', render: (_, program) => usedFor(program) },
    { title: t('settings.status'), key: 'status', render: (_, program) => status(program) },
    { title: t('settings.version'), key: 'version', render: (_, program) => location(program) },
    { title: t('settings.install'), key: 'install', render: (_, program) => install(program) },
  ]

  return (
    <Card>
      <PageHeader
        icon={<SettingOutlined />}
        title={t('settings.title')}
        subtitle={t('settings.toolsHint')}
        extra={
          <Button data-testid="tools-refresh-button" icon={<ReloadOutlined />} loading={refresh.isPending} onClick={() => refresh.mutate()}>
            {t('settings.refresh')}
          </Button>
        }
      />
      {unmet.length > 0 && (
        <div data-testid="tools-missing-alert">
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: token.marginMD }}
            message={t('settings.unmetTitle')}
            description={
              <ul style={{ margin: 0, paddingInlineStart: token.paddingMD }}>
                {unmet.map((need) => (
                  <li key={need.need} data-testid={`tools-unmet-${need.need}`}>
                    {t('settings.unmetNeed', { need: enumLabel(t, 'need', need.need), programs: need.missing.join(', ') })}
                  </li>
                ))}
              </ul>
            }
          />
        </div>
      )}
      <ResponsiveTable
        testId="tools-table"
        items={rows}
        loading={tools.isLoading}
        columns={columns}
        emptyText={EMPTY}
        pagination={false}
        cardTitle={(program) => program.name}
        cardExtra={status}
        cardTestId={(program) => `tool-card-${program.name}`}
        renderCard={(program) => (
          <Space direction="vertical" size={token.marginXS}>
            <span>{purpose(program)}</span>
            {requirement(program)}
            {usedFor(program)}
            {location(program)}
            {install(program)}
          </Space>
        )}
      />
    </Card>
  )
}

function SettingsCard() {
  const { t } = useTranslation()
  const [form] = Form.useForm<SettingsRequest>()
  const settings = useSettings()
  const save = useSaveSettings()

  useEffect(() => {
    if (settings.data) form.setFieldsValue(settings.data)
  }, [settings.data, form])

  const onFinish = (values: SettingsRequest) =>
    save.mutate(values, {
      onSuccess: () => message.success(t('settings.saved')),
      onError: (error) => {
        const unplaced = applyFormErrors(form, t, error, ['workers', 'scan_workers', 'nice'])
        if (unplaced) message.error(unplaced)
      },
    })

  return (
    <Card title={t('settings.globalTitle')} loading={settings.isLoading}>
      <Form form={form} layout="vertical" onFinish={onFinish} disabled={save.isPending} data-testid="settings-form" style={{ maxWidth: 480 }}>
        <Form.Item
          name="workers"
          label={t('settings.workers')}
          extra={t('settings.workersHint', { count: settings.data?.effective_workers ?? 1 })}
          rules={[{ required: true, message: t('common.required') }]}
        >
          <InputNumber data-testid="settings-workers-input" min={0} max={64} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="scan_workers"
          label={t('settings.scanWorkers')}
          extra={t('settings.scanWorkersHint')}
          rules={[{ required: true, message: t('common.required') }]}
        >
          <InputNumber data-testid="settings-scan-workers-input" min={1} max={16} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="nice" label={t('settings.nice')} extra={t('settings.niceHint')} valuePropName="checked">
          <Switch data-testid="settings-nice-switch" />
        </Form.Item>
        <Button data-testid="settings-save-button" type="primary" htmlType="submit" loading={save.isPending}>
          {t('common.save')}
        </Button>
      </Form>
    </Card>
  )
}
