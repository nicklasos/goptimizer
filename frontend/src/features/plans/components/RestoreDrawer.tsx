import { useState } from 'react'
import { Alert, Button, Checkbox, Col, Collapse, Descriptions, Drawer, Form, Grid, Input, InputNumber, Row, Select, Spin, Switch, Typography, theme } from 'antd'
import { RollbackOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { EMPTY, formatBytes, formatDuration, formatUtcDateTime } from '@/shared/utils/format'
import { applyFormErrors } from '@/shared/utils/formErrors'
import { errorMessage } from '@/shared/utils/serverErrors'
import { useRestore, useRestoreTarget, useStartRestore } from '../hooks'
import { DEFAULT_PORTS, SSL_MODES } from '../planForm'
import { formTarget, ownTarget, planSource, restoreFormValues, restoreRequest, targetSummary, type RestoreFormValues } from '../restore'
import { isRunActive } from '../runStatus'
import { objectName } from '../source'
import type { Archive, Plan, RestoreDetail } from '../types'
import { LogBox } from './LogBox'
import { RunStatusTag } from './RunStatusTag'

/** Fields the API may name in a validation error, as the form names them. */
const FIELDS = [
  'postgres.host', 'postgres.port', 'postgres.user', 'postgres.database', 'postgres.password', 'postgres.ssl_mode',
  'mysql.host', 'mysql.port', 'mysql.user', 'mysql.database', 'mysql.password',
  'sqlite.path', 'files.directory', 'confirm',
] as const

interface Props {
  plan: Plan
  /** The archive to restore: the drawer shows the form for it while no restore is open. */
  archive: Archive | undefined
  /** A restore to follow: the drawer shows its log. */
  restoreId: number | undefined
  onStarted: (restore: RestoreDetail) => void
  onClose: () => void
}

/** The restore form for an archive, which becomes the restore's live log once it is started. */
export function RestoreDrawer({ plan, archive, restoreId, onStarted, onClose }: Props) {
  const { t } = useTranslation()
  const screens = Grid.useBreakpoint()
  const restore = useRestore(restoreId)
  const following = restoreId != null

  return (
    <Drawer
      open={following || archive != null}
      onClose={onClose}
      width={screens.md ? 760 : '100%'}
      title={following ? t('restores.drawerTitle', { id: restoreId }) : t('restores.formTitle')}
      extra={following && restore.data && <RunStatusTag status={restore.data.status} testId="restore-drawer-status" />}
      destroyOnClose
    >
      <div data-testid="restore-drawer">
        {following ? <RestoreLog restore={restore.data} /> : archive && <RestoreForm key={archive.id} plan={plan} archive={archive} onStarted={onStarted} />}
      </div>
    </Drawer>
  )
}

function ArchiveFacts({ archive }: { archive: { object_key: string; created_at: string; size_bytes: number; storage_name: string } }) {
  const { t } = useTranslation()
  const screens = Grid.useBreakpoint()
  return (
    <Descriptions size="small" column={screens.md ? 2 : 1} data-testid="restore-archive">
      <Descriptions.Item label={t('archives.file')}>
        <span style={{ overflowWrap: 'anywhere' }}>{objectName(archive.object_key)}</span>
      </Descriptions.Item>
      <Descriptions.Item label={t('archives.created')}>{formatUtcDateTime(archive.created_at)}</Descriptions.Item>
      <Descriptions.Item label={t('archives.size')}>{formatBytes(archive.size_bytes)}</Descriptions.Item>
      <Descriptions.Item label={t('archives.storage')}>{archive.storage_name}</Descriptions.Item>
    </Descriptions>
  )
}

function RestoreForm({ plan, archive, onStarted }: { plan: Plan; archive: Archive; onStarted: (restore: RestoreDetail) => void }) {
  const { t } = useTranslation()
  const target = useRestoreTarget(archive.id)
  if (target.isLoading) return <Spin />
  if (target.isError || !target.data) {
    return (
      <div data-testid="restore-target-error">
        <Alert type="error" showIcon message={errorMessage(t, target.error)} />
      </div>
    )
  }
  return <RestoreFormBody plan={plan} archive={archive} initial={restoreFormValues(target.data)} onStarted={onStarted} />
}

interface BodyProps {
  plan: Plan
  archive: Archive
  initial: RestoreFormValues
  onStarted: (restore: RestoreDetail) => void
}

function RestoreFormBody({ plan, archive, initial, onStarted }: BodyProps) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const [form] = Form.useForm<RestoreFormValues>()
  const [values, setValues] = useState<RestoreFormValues>(initial)
  // Untouched, the API decides: create a database other than the plan's own.
  const [createChoice, setCreateChoice] = useState<boolean>()
  const [failure, setFailure] = useState<string | null>(null)
  const start = useStartRestore()
  const kind = plan.source_kind
  const isDatabase = kind === 'postgres' || kind === 'mysql'
  const { own, phrase } = ownTarget(kind, planSource(plan), formTarget(plan, values))
  const confirmed = !own || values.confirm === phrase

  const submit = (submitted: RestoreFormValues) => {
    setFailure(null)
    const body = restoreRequest(kind, submitted, isDatabase ? createChoice : undefined, own ? submitted.confirm : undefined)
    start.mutate({ archiveId: archive.id, body }, { onSuccess: onStarted, onError: (error) => setFailure(applyFormErrors(form, t, error, FIELDS)) })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginMD }}>
      <ArchiveFacts archive={archive} />
      <Form
        form={form}
        layout="vertical"
        initialValues={initial}
        onValuesChange={(_, all) => setValues(all)}
        onFinish={submit}
        requiredMark={false}
        data-testid="restore-form"
      >
        {isDatabase && <DatabaseTarget kind={kind} createDatabase={createChoice ?? !own} onCreateDatabase={setCreateChoice} />}
        {kind === 'sqlite' && <SQLiteTarget />}
        {kind === 'files' && <FilesTarget plan={plan} original={values.files?.original_locations ?? false} />}

        {own && (
          <div data-testid="restore-own-warning" style={{ marginBottom: token.marginMD }}>
            <Alert
              type="error"
              showIcon
              message={t('restores.own.title')}
              description={
                <>
                  <div style={{ marginBottom: token.marginSM }}>{t(`restores.own.${kind}`)}</div>
                  <Form.Item
                    name="confirm"
                    label={
                      <span>
                        {t('restores.own.type')} <Typography.Text code>{phrase}</Typography.Text>
                      </span>
                    }
                    style={{ marginBottom: 0 }}
                  >
                    <Input data-testid="restore-confirm-input" autoComplete="off" spellCheck={false} style={{ fontFamily: token.fontFamilyCode }} />
                  </Form.Item>
                </>
              }
            />
          </div>
        )}

        {failure && (
          <div data-testid="restore-form-error" style={{ marginBottom: token.marginMD }}>
            <Alert type="error" showIcon message={failure} />
          </div>
        )}
        <Button
          type="primary"
          danger={own}
          htmlType="submit"
          icon={<RollbackOutlined />}
          loading={start.isPending}
          disabled={!confirmed}
          block
          data-testid="restore-start-button"
        >
          {t('restores.start')}
        </Button>
      </Form>
    </div>
  )
}

function DatabaseTarget({ kind, createDatabase, onCreateDatabase }: { kind: 'postgres' | 'mysql'; createDatabase: boolean; onCreateDatabase: (value: boolean) => void }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const required = [{ required: true, whitespace: true, message: t('common.required') }]
  return (
    <>
      <Form.Item name={[kind, 'database']} label={t('restores.database')} extra={t('restores.databaseHint')} rules={required}>
        <Input data-testid="restore-database-input" size="large" autoComplete="off" spellCheck={false} style={{ fontFamily: token.fontFamilyCode }} />
      </Form.Item>
      <Form.Item>
        <Checkbox checked={createDatabase} onChange={(event) => onCreateDatabase(event.target.checked)} data-testid="restore-create-database-checkbox">
          {t('restores.createDatabase')}
        </Checkbox>
      </Form.Item>
      <Collapse
        ghost
        size="small"
        style={{ marginBottom: token.marginMD, marginInline: -token.paddingSM }}
        items={[
          {
            key: 'server',
            label: <span data-testid="restore-server-toggle">{t('restores.otherServer')}</span>,
            forceRender: true,
            children: (
              <>
                <Typography.Paragraph type="secondary">{t('restores.otherServerHint')}</Typography.Paragraph>
                <Row gutter={token.marginMD}>
                  <Col xs={24} sm={16}>
                    <Form.Item name={[kind, 'host']} label={t('plans.fields.host')} rules={required}>
                      <Input data-testid="restore-host-input" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} sm={8}>
                    <Form.Item name={[kind, 'port']} label={t('plans.fields.port')}>
                      <InputNumber data-testid="restore-port-input" min={1} max={65535} placeholder={String(DEFAULT_PORTS[kind])} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <Row gutter={token.marginMD}>
                  <Col xs={24} sm={12}>
                    <Form.Item name={[kind, 'user']} label={t('plans.fields.user')} rules={required}>
                      <Input data-testid="restore-user-input" autoComplete="off" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} sm={12}>
                    <Form.Item name={[kind, 'password']} label={t('plans.fields.password')}>
                      <Input.Password data-testid="restore-password-input" placeholder={t('restores.passwordKeep')} autoComplete="new-password" />
                    </Form.Item>
                  </Col>
                </Row>
                {kind === 'postgres' && (
                  <Form.Item name={['postgres', 'ssl_mode']} label={t('plans.fields.sslMode')} style={{ marginBottom: 0 }}>
                    <Select data-testid="restore-ssl-select" options={SSL_MODES.map((mode) => ({ value: mode, label: enumLabel(t, 'sslMode', mode) }))} />
                  </Form.Item>
                )}
              </>
            ),
          },
        ]}
      />
    </>
  )
}

const absolutePath = /^\s*\//

function SQLiteTarget() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  return (
    <Form.Item
      name={['sqlite', 'path']}
      label={t('restores.sqlitePath')}
      extra={t('restores.sqlitePathHint')}
      rules={[
        { required: true, whitespace: true, message: t('common.required') },
        { pattern: absolutePath, message: t('plans.fields.absolutePath') },
      ]}
    >
      <Input data-testid="restore-sqlite-path-input" spellCheck={false} style={{ fontFamily: token.fontFamilyCode }} />
    </Form.Item>
  )
}

function FilesTarget({ plan, original }: { plan: Plan; original: boolean }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  return (
    <>
      <Form.Item
        name={['files', 'directory']}
        label={t('restores.directory')}
        extra={t('restores.directoryHint')}
        rules={original ? [] : [{ required: true, whitespace: true, message: t('common.required') }, { pattern: absolutePath, message: t('plans.fields.absolutePath') }]}
      >
        <Input data-testid="restore-directory-input" disabled={original} spellCheck={false} style={{ fontFamily: token.fontFamilyCode }} />
      </Form.Item>
      <Form.Item>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: token.marginXS }}>
          <Form.Item name={['files', 'original_locations']} valuePropName="checked" noStyle>
            <Switch data-testid="restore-original-switch" />
          </Form.Item>
          {t('restores.originalLocations')}
        </span>
      </Form.Item>
      {original && (
        <Typography.Paragraph type="secondary">
          {t('restores.originalPaths')} <Typography.Text code>{plan.files?.paths.join(', ')}</Typography.Text>
        </Typography.Paragraph>
      )}
    </>
  )
}

function RestoreLog({ restore }: { restore: RestoreDetail | undefined }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const screens = Grid.useBreakpoint()
  if (!restore) return <Spin />
  const summary = targetSummary(restore.target)
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginMD }}>
      {restore.error && (
        <div data-testid="restore-error">
          <Alert type="error" showIcon message={restore.error} />
        </div>
      )}
      {restore.archive ? <ArchiveFacts archive={restore.archive} /> : <Typography.Text type="secondary">{t('restores.archiveGone')}</Typography.Text>}
      <Descriptions size="small" column={screens.md ? 2 : 1}>
        <Descriptions.Item label={t('restores.target')} span={screens.md ? 2 : 1}>
          <Typography.Text code data-testid="restore-target-summary" style={{ overflowWrap: 'anywhere' }}>
            {summary ?? t('restores.originalLocationsShort')}
          </Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('runs.started')}>{formatUtcDateTime(restore.started_at)}</Descriptions.Item>
        <Descriptions.Item label={t('runs.finished')}>{formatUtcDateTime(restore.finished_at)}</Descriptions.Item>
        <Descriptions.Item label={t('runs.duration')}>{formatDuration(restore.duration_ms)}</Descriptions.Item>
        <Descriptions.Item label={t('restores.by')}>{restore.created_by?.name || restore.created_by?.email || EMPTY}</Descriptions.Item>
      </Descriptions>
      <LogBox log={restore.log} active={isRunActive(restore)} testId="restore-log" />
    </div>
  )
}
