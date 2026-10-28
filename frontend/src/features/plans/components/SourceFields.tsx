import { useState } from 'react'
import { Button, Col, Form, Input, InputNumber, Row, Select, Space, theme, type FormInstance } from 'antd'
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { DEFAULT_PORTS, parseConnectionUrl, SSL_MODES, type PlanFormValues } from '../planForm'

interface Props {
  form: FormInstance<PlanFormValues>
  kind: PlanFormValues['source_kind']
  /** A password is stored for this kind, so the password field may stay blank. */
  secretStored: boolean
  /** setFieldsValue does not count as an edit, so the URL helper reports its own. */
  onFilled: () => void
}

const absolutePath = /^\s*\//

/** The fields of the chosen source kind. */
export function SourceFields({ form, kind, secretStored, onFilled }: Props) {
  if (kind === 'postgres' || kind === 'mysql') return <DatabaseFields form={form} kind={kind} secretStored={secretStored} onFilled={onFilled} />
  if (kind === 'sqlite') return <SQLiteFields />
  return <FilesFields />
}

function DatabaseFields({ form, kind, secretStored, onFilled }: Props & { kind: 'postgres' | 'mysql' }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const required = [{ required: true, whitespace: true, message: t('common.required') }]
  const [url, setUrl] = useState('')
  const [urlState, setUrlState] = useState<'idle' | 'filled' | 'invalid'>('idle')

  // The URL is read into the fields and then cleared, so its password is never kept or sent.
  const applyUrl = (raw: string) => {
    if (!raw.trim()) return
    const parsed = parseConnectionUrl(raw)
    if (!parsed) {
      setUrlState('invalid')
      return
    }
    const { kind: parsedKind, ssl_mode, ...connection } = parsed
    form.setFieldsValue(
      parsedKind === 'postgres'
        ? { source_kind: parsedKind, postgres: { ...connection, ssl_mode: ssl_mode ?? 'disable' } }
        : { source_kind: parsedKind, mysql: connection }
    )
    setUrl('')
    setUrlState('filled')
    onFilled()
  }

  return (
    <>
      <Form.Item
        label={t('plans.url.label')}
        extra={urlState === 'idle' ? t('plans.url.hint') : undefined}
        validateStatus={urlState === 'invalid' ? 'error' : urlState === 'filled' ? 'success' : undefined}
        help={urlState === 'invalid' ? t('plans.url.invalid') : urlState === 'filled' ? t('plans.url.filled') : undefined}
      >
        <Input
          data-testid="plan-source-url-input"
          value={url}
          autoComplete="off"
          spellCheck={false}
          placeholder={kind === 'postgres' ? 'postgres://user:password@host:5432/database' : 'mysql://user:password@host:3306/database'}
          style={{ fontFamily: token.fontFamilyCode }}
          onChange={(event) => {
            setUrl(event.target.value)
            setUrlState('idle')
          }}
          onPaste={(event) => {
            event.preventDefault()
            applyUrl(event.clipboardData.getData('text'))
          }}
          onPressEnter={(event) => {
            event.preventDefault()
            applyUrl(url)
          }}
          onBlur={() => applyUrl(url)}
        />
      </Form.Item>

      <Row gutter={token.marginMD}>
        <Col xs={24} sm={16}>
          <Form.Item name={[kind, 'host']} label={t('plans.fields.host')} rules={required}>
            <Input data-testid="plan-db-host-input" placeholder="127.0.0.1" />
          </Form.Item>
        </Col>
        <Col xs={24} sm={8}>
          <Form.Item name={[kind, 'port']} label={t('plans.fields.port')}>
            <InputNumber data-testid="plan-db-port-input" min={1} max={65535} placeholder={String(DEFAULT_PORTS[kind])} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={token.marginMD}>
        <Col xs={24} sm={12}>
          <Form.Item name={[kind, 'database']} label={t('plans.fields.database')} rules={required}>
            <Input data-testid="plan-db-database-input" />
          </Form.Item>
        </Col>
        <Col xs={24} sm={12}>
          <Form.Item name={[kind, 'user']} label={t('plans.fields.user')} rules={required}>
            <Input data-testid="plan-db-user-input" autoComplete="off" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={token.marginMD}>
        <Col xs={24} sm={kind === 'postgres' ? 12 : 24}>
          <Form.Item name={[kind, 'password']} label={t('plans.fields.password')} rules={secretStored ? [] : [{ required: true, message: t('common.required') }]}>
            <Input.Password
              data-testid="plan-db-password-input"
              placeholder={secretStored ? t('plans.fields.passwordKeep') : undefined}
              autoComplete="new-password"
            />
          </Form.Item>
        </Col>
        {kind === 'postgres' && (
          <Col xs={24} sm={12}>
            <Form.Item name={['postgres', 'ssl_mode']} label={t('plans.fields.sslMode')}>
              <Select
                data-testid="plan-db-ssl-select"
                options={SSL_MODES.map((mode) => ({ value: mode, label: enumLabel(t, 'sslMode', mode) }))}
              />
            </Form.Item>
          </Col>
        )}
      </Row>
    </>
  )
}

function SQLiteFields() {
  const { t } = useTranslation()
  return (
    <Form.Item
      name={['sqlite', 'path']}
      label={t('plans.fields.sqlitePath')}
      extra={t('plans.fields.sqlitePathHint')}
      rules={[
        { required: true, whitespace: true, message: t('common.required') },
        { pattern: absolutePath, message: t('plans.fields.absolutePath') },
      ]}
    >
      <Input data-testid="plan-sqlite-path-input" placeholder="/var/lib/myproject/app.db" />
    </Form.Item>
  )
}

function FilesFields() {
  const { t } = useTranslation()
  return (
    <>
      <PathList
        name="paths"
        label={t('plans.fields.paths')}
        hint={t('plans.fields.pathsHint')}
        addLabel={t('plans.fields.addPath')}
        placeholder="/var/www/myproject/uploads"
        testId="plan-files-path"
        minRows={1}
        rules={[
          { required: true, whitespace: true, message: t('common.required') },
          { pattern: absolutePath, message: t('plans.fields.absolutePath') },
        ]}
      />
      <PathList
        name="excludes"
        label={t('plans.fields.excludes')}
        hint={t('plans.fields.excludesHint')}
        addLabel={t('plans.fields.addExclude')}
        placeholder="*.log"
        testId="plan-files-exclude"
        minRows={0}
        rules={[]}
      />
    </>
  )
}

interface PathListProps {
  name: 'paths' | 'excludes'
  label: string
  hint: string
  addLabel: string
  placeholder: string
  testId: string
  minRows: number
  rules: { required?: boolean; whitespace?: boolean; pattern?: RegExp; message: string }[]
}

/** An editable list of lines: a minus beside each row, a plus under them. */
function PathList({ name, label, hint, addLabel, placeholder, testId, minRows, rules }: PathListProps) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  return (
    <Form.Item label={label} extra={hint} required={minRows > 0}>
      <Form.List name={['files', name]}>
        {(fields, { add, remove }) => (
          <Space direction="vertical" size={token.marginXS} style={{ width: '100%' }}>
            {fields.map((field, index) => (
              <div key={field.key} style={{ display: 'flex', gap: token.marginXS, alignItems: 'flex-start' }}>
                <Form.Item name={field.name} rules={rules} style={{ flex: 1, marginBottom: 0 }}>
                  <Input data-testid={`${testId}-input-${index}`} placeholder={placeholder} spellCheck={false} />
                </Form.Item>
                <Button
                  type="text"
                  icon={<MinusCircleOutlined />}
                  aria-label={t('plans.fields.remove')}
                  disabled={fields.length <= minRows}
                  onClick={() => remove(field.name)}
                  data-testid={`${testId}-remove-${index}`}
                />
              </div>
            ))}
            <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={() => add('')} data-testid={`${testId}-add`}>
              {addLabel}
            </Button>
          </Space>
        )}
      </Form.List>
    </Form.Item>
  )
}
