import { useEffect, useState } from 'react'
import { Alert, Button, Card, Col, Collapse, Form, Input, InputNumber, Popconfirm, Row, Segmented, Select, Space, theme } from 'antd'
import { FolderOutlined } from '@ant-design/icons'
import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { useGuardedNavigate, useUnsavedChangesContext, useUnsavedChangesGuard } from '@/shared/lib/unsavedChanges'
import { enumLabel } from '@/shared/utils/enums'
import { applyFormErrors } from '@/shared/utils/formErrors'
import message from '@/shared/utils/message'
import { FOLDER_FORM_FIELDS, folderToForm, formToRequest, NEW_FOLDER, type FolderFormValues } from '../folderForm'
import { useDeleteFolder, useFolder, useSaveFolder } from '../hooks'
import { MODES, type Mode } from '../types'

const COMPRESSION = 'compression'

/** `/folders/new` and `/folders/:id/edit`. */
export default function FolderEditorPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const params = useParams()
  const id = params.id ? Number(params.id) : undefined
  const navigate = useGuardedNavigate()
  const { setDirty: setGuardDirty } = useUnsavedChangesContext()
  const [form] = Form.useForm<FolderFormValues>()
  const mode = Form.useWatch(['profile', 'mode'], form) ?? NEW_FOLDER.profile.mode
  const [dirty, setDirty] = useState(false)
  const [compressionOpen, setCompressionOpen] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)
  const folder = useFolder(id)
  const save = useSaveFolder()
  const remove = useDeleteFolder()

  useUnsavedChangesGuard(dirty)

  useEffect(() => {
    if (folder.data) form.setFieldsValue(folderToForm(folder.data))
  }, [folder.data, form])

  const leave = () => {
    setDirty(false)
    setGuardDirty(false)
    navigate('/')
  }

  // The compression section is folded by default; an error inside it would be invisible.
  const revealErrors = () => {
    if (form.getFieldsError().some((field) => field.errors.length && field.name[0] === 'profile')) setCompressionOpen(true)
  }

  const back = { label: t('folders.title'), onClick: () => navigate('/'), testId: 'folder-editor-back-button' }

  if (id != null && folder.isLoading) return <PageLoading />
  if (id != null && folder.isError) return <PageError error={folder.error} back={back} onRetry={() => folder.refetch()} />

  // getFieldsValue(true) keeps the lossy-only fields, which are not rendered in lossless mode.
  const onFinish = () => {
    setServerError(null)
    save.mutate(
      { id, body: formToRequest(form.getFieldsValue(true)) },
      {
        onSuccess: () => {
          message.success(t('folders.saved'))
          leave()
        },
        onError: (error) => {
          setServerError(applyFormErrors(form, t, error, FOLDER_FORM_FIELDS))
          revealErrors()
        },
      }
    )
  }

  const onDelete = () => {
    if (id == null) return
    remove.mutate(id, {
      onSuccess: () => {
        message.success(t('folders.deleted'))
        leave()
      },
    })
  }

  const lossless = mode === 'lossless'
  const number = (name: string, label: string, testId: string, props: { min: number; max: number; step?: number; addon?: string }) => (
    <Form.Item name={['profile', name]} label={label} rules={[{ required: true, message: t('common.required') }]}>
      <InputNumber data-testid={testId} min={props.min} max={props.max} step={props.step} addonAfter={props.addon} style={{ width: '100%' }} />
    </Form.Item>
  )

  return (
    <PageStack testId="folder-editor-page">
      <Card>
        <PageHeader icon={<FolderOutlined />} title={id == null ? t('folders.add') : t('folders.edit')} back={back} />

        {serverError && (
          <Alert data-testid="folder-editor-error" type="error" showIcon message={serverError} style={{ marginBottom: token.marginMD }} />
        )}

        <Form
          form={form}
          layout="vertical"
          initialValues={NEW_FOLDER}
          onFinish={onFinish}
          onFinishFailed={revealErrors}
          onValuesChange={() => setDirty(true)}
          disabled={save.isPending}
          data-testid="folder-editor-form"
          style={{ maxWidth: 720 }}
        >
          <Form.Item name="name" label={t('folders.name')} rules={[{ required: true, whitespace: true, message: t('common.required') }]}>
            <Input data-testid="folder-name-input" maxLength={100} placeholder={t('folders.namePlaceholder')} />
          </Form.Item>

          <Form.Item
            name="path"
            label={t('folders.path')}
            extra={t('folders.pathHint')}
            rules={[{ required: true, whitespace: true, message: t('common.required') }]}
          >
            <Input data-testid="folder-path-input" placeholder="/var/www/example.com/public/uploads" spellCheck={false} />
          </Form.Item>

          <Row gutter={token.marginMD}>
            <Col xs={24} sm={12}>
              <Form.Item
                name="scan_interval_minutes"
                label={t('folders.scanInterval')}
                extra={t('folders.scanIntervalHint')}
                rules={[{ required: true, message: t('common.required') }]}
              >
                <InputNumber data-testid="folder-scan-interval-input" min={5} max={10080} addonAfter={t('folders.minutes')} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12}>
              <Form.Item
                name="full_scan_interval_hours"
                label={t('folders.fullScanInterval')}
                extra={t('folders.fullScanIntervalHint')}
                rules={[{ required: true, message: t('common.required') }]}
              >
                <InputNumber data-testid="folder-full-scan-interval-input" min={1} max={720} addonAfter={t('folders.hours')} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Collapse
            activeKey={compressionOpen ? [COMPRESSION] : []}
            onChange={(keys) => setCompressionOpen(keys.includes(COMPRESSION))}
            style={{ marginBottom: token.marginLG }}
            items={[
              {
                key: COMPRESSION,
                forceRender: true,
                label: <span data-testid="folder-compression-toggle">{t('folders.compression')}</span>,
                children: (
                  <>
                    <Form.Item name={['profile', 'mode']} label={t('folders.mode')} extra={t(`folders.modeHint.${mode}`)}>
                      <Segmented<Mode>
                        data-testid="folder-mode-picker"
                        options={MODES.map((value) => ({
                          value,
                          label: <span data-testid={`folder-mode-option-${value}`}>{enumLabel(t, 'mode', value)}</span>,
                        }))}
                      />
                    </Form.Item>
                    {!lossless && (
                      <Row gutter={token.marginMD}>
                        <Col xs={24} sm={12}>
                          {number('jpeg_max_quality', t('folders.jpegMaxQuality'), 'folder-jpeg-quality-input', { min: 40, max: 100 })}
                        </Col>
                        <Col xs={24} sm={12}>
                          {number('gif_lossy', t('folders.gifLossy'), 'folder-gif-lossy-input', { min: 0, max: 200 })}
                        </Col>
                        <Col xs={24} sm={12}>
                          {number('png_quality_min', t('folders.pngQualityMin'), 'folder-png-quality-min-input', { min: 0, max: 100 })}
                        </Col>
                        <Col xs={24} sm={12}>
                          {number('png_quality_max', t('folders.pngQualityMax'), 'folder-png-quality-max-input', { min: 0, max: 100 })}
                        </Col>
                        <Col xs={24} sm={12}>
                          {number('min_ssim', t('folders.minSsim'), 'folder-min-ssim-input', { min: 0.5, max: 1, step: 0.01 })}
                        </Col>
                      </Row>
                    )}
                    <Row gutter={token.marginMD}>
                      <Col xs={24} sm={12}>
                        {number('min_saving_percent', t('folders.minSavingPercent'), 'folder-min-saving-input', { min: 0, max: 90, addon: '%' })}
                      </Col>
                      <Col xs={24} sm={12}>
                        {number('min_age_minutes', t('folders.minAgeMinutes'), 'folder-min-age-input', { min: 0, max: 1440, addon: t('folders.minutes') })}
                      </Col>
                    </Row>
                    <Form.Item name={['profile', 'excludes']} label={t('folders.excludes')} extra={t('folders.excludesHint')} style={{ marginBottom: 0 }}>
                      <Select data-testid="folder-excludes-select" mode="tags" open={false} tokenSeparators={[',', ' ']} placeholder="cache/**" />
                    </Form.Item>
                  </>
                ),
              },
            ]}
          />

          <Space wrap>
            <Button data-testid="folder-save-button" type="primary" htmlType="submit" loading={save.isPending}>
              {t('common.save')}
            </Button>
            {id != null && (
              <Popconfirm
                title={t('folders.deleteConfirm')}
                description={t('folders.deleteHint')}
                onConfirm={onDelete}
                okText={t('common.yes')}
                cancelText={t('common.no')}
                okButtonProps={{ danger: true, 'data-testid': 'folder-delete-confirm' }}
              >
                <Button data-testid="folder-delete-button" danger loading={remove.isPending}>
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
