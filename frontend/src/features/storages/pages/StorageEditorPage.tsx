import { useEffect, useState, type ReactNode } from 'react'
import { Alert, Button, Card, Checkbox, Col, Form, Grid, Input, Popconfirm, Row, Segmented, Space, Typography, Upload, theme } from 'antd'
import { ApiOutlined, CloudOutlined, CloudServerOutlined, FolderOutlined, UploadOutlined } from '@ant-design/icons'
import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { useGuardedNavigate, useUnsavedChangesContext, useUnsavedChangesGuard } from '@/shared/lib/unsavedChanges'
import { enumLabel } from '@/shared/utils/enums'
import message from '@/shared/utils/message'
import { applyFormErrors } from '@/shared/utils/formErrors'
import { errorMessage } from '@/shared/utils/serverErrors'
import { useDeleteStorage, useSaveStorage, useStorage, useTestStorage } from '../hooks'
import { plansUsingStorage } from '../inUse'
import { emptyStorageForm, formToRequest, hasStoredSecret, STORAGE_FORM_FIELDS, storageToForm, type StorageFormValues } from '../storageForm'
import { STORAGE_KINDS, type StorageKind, type StorageTestResult } from '../types'

const KIND_ICONS: Record<StorageKind, ReactNode> = {
  local: <FolderOutlined />,
  s3: <CloudServerOutlined />,
  gcs: <CloudOutlined />,
}

export default function StorageEditorPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const screens = Grid.useBreakpoint()
  const params = useParams()
  const id = params.id ? Number(params.id) : undefined
  const navigate = useGuardedNavigate()
  const { setDirty: setGuardDirty } = useUnsavedChangesContext()
  const [form] = Form.useForm<StorageFormValues>()
  const kind = Form.useWatch('kind', form) ?? 'local'
  const [dirty, setDirty] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<StorageTestResult | null>(null)
  const storage = useStorage(id)
  const save = useSaveStorage()
  const test = useTestStorage()
  const remove = useDeleteStorage()

  useUnsavedChangesGuard(dirty)

  useEffect(() => {
    if (storage.data) form.setFieldsValue(storageToForm(storage.data))
  }, [storage.data, form])

  const leave = () => {
    setDirty(false)
    setGuardDirty(false)
    navigate('/storages')
  }

  const back = { label: t('storages.title'), onClick: () => navigate('/storages'), testId: 'storage-editor-back-button' }

  if (id != null && storage.isLoading) return <PageLoading />
  if (id != null && storage.isError) return <PageError error={storage.error} back={back} onRetry={() => storage.refetch()} />

  const secretStored = hasStoredSecret(storage.data, kind)
  const secretPlaceholder = secretStored ? t('storages.secretKeepPlaceholder') : undefined
  const secretRules = secretStored ? [] : [{ required: true, whitespace: true, message: t('common.required') }]

  // getFieldsValue(true) includes the blocks of the other kinds; formToRequest keeps only the chosen one.
  const onFinish = () => {
    setServerError(null)
    save.mutate(
      { id, body: formToRequest(form.getFieldsValue(true)) },
      {
        onSuccess: () => {
          message.success(t('storages.saved'))
          leave()
        },
        onError: (error) => setServerError(applyFormErrors(form, t, error, STORAGE_FORM_FIELDS)),
      }
    )
  }

  const onTest = async () => {
    setServerError(null)
    setTestResult(null)
    try {
      await form.validateFields()
    } catch {
      return
    }
    test.mutate(
      { storageId: id, body: formToRequest(form.getFieldsValue(true)) },
      {
        onSuccess: setTestResult,
        onError: (error) => setServerError(applyFormErrors(form, t, error, STORAGE_FORM_FIELDS)),
      }
    )
  }

  const onDelete = () => {
    if (id == null) return
    setServerError(null)
    remove.mutate(id, {
      onSuccess: () => {
        message.success(t('storages.deleted'))
        leave()
      },
      onError: (error) => {
        const plans = plansUsingStorage(error)
        setServerError(plans ? t('storages.inUse', { plans: plans.map((plan) => plan.name).join(', ') }) : errorMessage(t, error))
      },
    })
  }

  const loadKeyFile = async (file: File) => {
    form.setFieldValue(['gcs', 'credentials_json'], await file.text())
    setDirty(true)
    return false
  }

  return (
    <PageStack testId="storage-editor-page">
      <Card>
        <PageHeader
          title={id == null ? t('storages.create') : t('storages.edit')}
          back={back}
          extra={
            id != null && (
              <Button data-testid="storage-editor-browse-button" onClick={() => navigate(`/storages/${id}/browse`)}>
                {t('storages.browse')}
              </Button>
            )
          }
        />

        {serverError && (
          <Alert data-testid="storage-editor-error" type="error" showIcon message={serverError} style={{ marginBottom: token.marginMD }} />
        )}

        <Form
          form={form}
          layout="vertical"
          initialValues={emptyStorageForm()}
          onFinish={onFinish}
          onValuesChange={() => {
            setDirty(true)
            setTestResult(null)
          }}
          disabled={save.isPending}
          data-testid="storage-editor-form"
          style={{ maxWidth: 720 }}
        >
          <Form.Item name="name" label={t('storages.nameField')} rules={[{ required: true, whitespace: true, message: t('storages.nameRequired') }]}>
            <Input data-testid="storage-name-input" maxLength={100} placeholder={t('storages.namePlaceholder')} />
          </Form.Item>

          <Form.Item name="kind" label={t('storages.kindField')}>
            <Segmented<StorageKind>
              block
              vertical={!screens.sm}
              data-testid="storage-kind-picker"
              options={STORAGE_KINDS.map((value) => ({
                value,
                icon: KIND_ICONS[value],
                label: <span data-testid={`storage-kind-option-${value}`}>{enumLabel(t, 'storageKind', value)}</span>,
              }))}
            />
          </Form.Item>

          <Typography.Paragraph type="secondary" data-testid="storage-kind-hint">
            {t(`storages.kindHint.${kind}`)}
          </Typography.Paragraph>

          {kind === 'local' && (
            <Form.Item
              name={['local', 'path']}
              label={t('storages.local.path')}
              extra={t('storages.local.pathHint')}
              rules={[
                { required: true, whitespace: true, message: t('common.required') },
                { pattern: /^\s*\//, message: t('errors.validation.local.path.invalid') },
              ]}
            >
              <Input data-testid="storage-local-path-input" placeholder="/var/backups/gogobackup" />
            </Form.Item>
          )}

          {kind === 's3' && (
            <>
              <Form.Item name={['s3', 'endpoint']} label={t('storages.s3.endpoint')} extra={t('storages.s3.endpointHint')}>
                <Input data-testid="storage-s3-endpoint-input" placeholder="https://s3.eu-central-1.amazonaws.com" />
              </Form.Item>
              <Row gutter={token.marginMD}>
                <Col xs={24} sm={14}>
                  <Form.Item name={['s3', 'bucket']} label={t('storages.s3.bucket')} rules={[{ required: true, whitespace: true, message: t('common.required') }]}>
                    <Input data-testid="storage-s3-bucket-input" />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={10}>
                  <Form.Item name={['s3', 'region']} label={t('storages.s3.region')}>
                    <Input data-testid="storage-s3-region-input" placeholder="us-east-1" />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item name={['s3', 'prefix']} label={t('storages.prefix')} extra={t('storages.prefixHint')}>
                <Input data-testid="storage-s3-prefix-input" placeholder="gogobackup/" />
              </Form.Item>
              <Form.Item name={['s3', 'use_path_style']} valuePropName="checked" extra={t('storages.s3.pathStyleHint')}>
                <Checkbox data-testid="storage-s3-path-style-checkbox">{t('storages.s3.pathStyle')}</Checkbox>
              </Form.Item>
              <Form.Item name={['s3', 'access_key_id']} label={t('storages.s3.accessKeyId')} rules={secretRules}>
                <Input data-testid="storage-s3-access-key-input" placeholder={secretPlaceholder} autoComplete="off" />
              </Form.Item>
              <Form.Item name={['s3', 'secret_access_key']} label={t('storages.s3.secretAccessKey')} rules={secretRules}>
                <Input.Password data-testid="storage-s3-secret-key-input" placeholder={secretPlaceholder} autoComplete="new-password" />
              </Form.Item>
            </>
          )}

          {kind === 'gcs' && (
            <>
              <Form.Item name={['gcs', 'bucket']} label={t('storages.gcs.bucket')} rules={[{ required: true, whitespace: true, message: t('common.required') }]}>
                <Input data-testid="storage-gcs-bucket-input" />
              </Form.Item>
              <Form.Item name={['gcs', 'prefix']} label={t('storages.prefix')} extra={t('storages.prefixHint')}>
                <Input data-testid="storage-gcs-prefix-input" placeholder="gogobackup/" />
              </Form.Item>
              <Form.Item
                name={['gcs', 'credentials_json']}
                label={t('storages.gcs.credentials')}
                extra={t('storages.gcs.credentialsHint')}
                rules={secretRules}
              >
                <Input.TextArea
                  data-testid="storage-gcs-credentials-input"
                  placeholder={secretPlaceholder ?? '{ "type": "service_account", ... }'}
                  autoSize={{ minRows: 3, maxRows: 10 }}
                  spellCheck={false}
                  style={{ fontFamily: token.fontFamilyCode }}
                />
              </Form.Item>
              <Form.Item>
                <Upload accept="application/json,.json" showUploadList={false} beforeUpload={loadKeyFile}>
                  <Button icon={<UploadOutlined />} data-testid="storage-gcs-credentials-file-button">
                    {t('storages.gcs.loadFile')}
                  </Button>
                </Upload>
              </Form.Item>
            </>
          )}

          <Form.Item>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Button icon={<ApiOutlined />} onClick={onTest} loading={test.isPending} data-testid="storage-test-button">
                {t('storages.testConnection')}
              </Button>
              {testResult && <TestResultAlert result={testResult} />}
            </Space>
          </Form.Item>

          <Space wrap>
            <Button data-testid="storage-save-button" type="primary" htmlType="submit" loading={save.isPending}>
              {t('common.save')}
            </Button>
            {id != null && (
              <Popconfirm
                title={t('storages.deleteConfirm')}
                description={t('storages.deleteHint')}
                onConfirm={onDelete}
                okText={t('common.yes')}
                cancelText={t('common.no')}
                okButtonProps={{ danger: true, 'data-testid': 'storage-editor-delete-confirm' }}
              >
                <Button data-testid="storage-editor-delete-button" danger loading={remove.isPending}>
                  {t('common.delete')}
                </Button>
              </Popconfirm>
            )}
          </Space>
        </Form>
      </Card>
    </PageStack>
  )
}

function TestResultAlert({ result }: { result: StorageTestResult }) {
  const { t } = useTranslation()
  if (result.ok) {
    return (
      <div data-testid="storage-test-ok">
        <Alert type="success" showIcon message={t('storages.testOk')} />
      </div>
    )
  }
  const step = result.step ?? 'connect'
  return (
    <div data-testid="storage-test-failed" data-step={step}>
      <Alert
        type="error"
        showIcon
        message={t('storages.testFailed', { step: enumLabel(t, 'storageTestStep', step) })}
        description={<Typography.Text style={{ wordBreak: 'break-word' }}>{result.error}</Typography.Text>}
      />
    </div>
  )
}
