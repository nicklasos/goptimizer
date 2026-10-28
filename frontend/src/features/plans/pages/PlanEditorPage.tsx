import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Alert, Button, Card, Divider, Form, Grid, Input, Popconfirm, Segmented, Select, Space, Switch, Tag, Typography, theme } from 'antd'
import { ApiOutlined, DatabaseOutlined, FileOutlined, FolderOutlined, PlusOutlined } from '@ant-design/icons'
import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { useGuardedNavigate, useUnsavedChangesContext, useUnsavedChangesGuard } from '@/shared/lib/unsavedChanges'
import { enumLabel } from '@/shared/utils/enums'
import { applyFormErrors } from '@/shared/utils/formErrors'
import message from '@/shared/utils/message'
import { RetentionFields } from '../components/RetentionFields'
import { ScheduleRows } from '../components/ScheduleRows'
import { SourceFields } from '../components/SourceFields'
import { SourceTestResult } from '../components/SourceTestResult'
import { useDeletePlan, usePlan, usePlanStorageChoices, useSavePlan, useTestPlanSource } from '../hooks'
import { emptyPlanForm, formToRequest, formToSource, hasStoredSecret, planFormFields, planToForm, sourceFieldPaths, type PlanFormValues } from '../planForm'
import { duplicateSchedules } from '../schedule'
import { SOURCE_KINDS, type SourceKind, type SourceTestResult as TestResult } from '../types'

const KIND_ICONS: Record<SourceKind, ReactNode> = {
  postgres: <DatabaseOutlined />,
  mysql: <DatabaseOutlined />,
  sqlite: <FileOutlined />,
  files: <FolderOutlined />,
}

export default function PlanEditorPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const screens = Grid.useBreakpoint()
  const params = useParams()
  const id = params.id ? Number(params.id) : undefined
  const navigate = useGuardedNavigate()
  const { setDirty: setGuardDirty } = useUnsavedChangesContext()
  const [form] = Form.useForm<PlanFormValues>()
  const values = Form.useWatch((all: PlanFormValues) => all, { form, preserve: true }) ?? emptyPlanForm()
  const kind = values.source_kind ?? 'postgres'
  const [dirty, setDirty] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const plan = usePlan(id)
  const storages = usePlanStorageChoices()
  const save = useSavePlan()
  const test = useTestPlanSource()
  const remove = useDeletePlan()

  useUnsavedChangesGuard(dirty)

  // Once per plan: the plan is fetched again while a run is in progress, and that must not
  // overwrite what is being edited.
  const filledFor = useRef<number | null>(null)
  useEffect(() => {
    if (plan.data && filledFor.current !== plan.data.id) {
      form.setFieldsValue(planToForm(plan.data))
      filledFor.current = plan.data.id
    }
  }, [plan.data, form])

  const edited = () => {
    setDirty(true)
    setTestResult(null)
  }

  const leave = (to: string) => {
    setDirty(false)
    setGuardDirty(false)
    navigate(to)
  }

  const back =
    id == null
      ? { label: t('navigation.dashboard'), onClick: () => navigate('/'), testId: 'plan-editor-back-button' }
      : { label: plan.data?.name ?? t('common.back'), onClick: () => navigate(`/plans/${id}`), testId: 'plan-editor-back-button' }

  if (id != null && plan.isLoading) return <PageLoading />
  if (id != null && plan.isError) return <PageError error={plan.error} back={back} onRetry={() => plan.refetch()} />

  // getFieldsValue(true) includes the blocks of the other kinds; formToRequest keeps only the chosen one.
  const onFinish = () => {
    setServerError(null)
    const all = form.getFieldsValue(true) as PlanFormValues
    if (duplicateSchedules(all.schedules).length > 0) return
    save.mutate(
      { id, body: formToRequest(all) },
      {
        onSuccess: (saved) => {
          message.success(t('plans.saved'))
          leave(`/plans/${saved.id}`)
        },
        onError: (error) => setServerError(applyFormErrors(form, t, error, planFormFields(all))),
      }
    )
  }

  const onTest = async () => {
    setServerError(null)
    setTestResult(null)
    const all = form.getFieldsValue(true) as PlanFormValues
    try {
      await form.validateFields(sourceFieldPaths(all))
    } catch {
      return
    }
    test.mutate(
      { planId: id, body: formToSource(all) },
      {
        onSuccess: setTestResult,
        onError: (error) => setServerError(applyFormErrors(form, t, error, planFormFields(all))),
      }
    )
  }

  const onDelete = () => {
    if (id == null) return
    remove.mutate(id, {
      onSuccess: () => {
        message.success(t('plans.deleted'))
        leave('/')
      },
    })
  }

  const section = (title: string) => (
    <Divider orientation="left" orientationMargin={0} style={{ marginTop: token.marginLG }}>
      <Typography.Text strong>{title}</Typography.Text>
    </Divider>
  )

  const storageOptions = (storages.data?.data ?? []).map((storage) => ({
    value: storage.id,
    title: storage.name,
    label: (
      <Space size={token.marginXS}>
        {storage.name}
        <Tag bordered={false} style={{ marginInlineEnd: 0 }}>
          {enumLabel(t, 'storageKind', storage.kind)}
        </Tag>
      </Space>
    ),
  }))

  return (
    <PageStack testId="plan-editor-page">
      <Card>
        <PageHeader
          title={id == null ? t('plans.create') : t('plans.edit')}
          back={back}
        />

        {serverError && <Alert data-testid="plan-editor-error" type="error" showIcon message={serverError} style={{ marginBottom: token.marginMD }} />}

        <Form
          form={form}
          layout="vertical"
          initialValues={emptyPlanForm()}
          onFinish={onFinish}
          onValuesChange={edited}
          disabled={save.isPending}
          data-testid="plan-editor-form"
          style={{ maxWidth: 760 }}
        >
          <Form.Item name="name" label={t('plans.nameField')} rules={[{ required: true, whitespace: true, message: t('plans.nameRequired') }]}>
            <Input data-testid="plan-name-input" maxLength={100} placeholder={t('plans.namePlaceholder')} />
          </Form.Item>

          {section(t('plans.sections.what'))}
          <Form.Item name="source_kind">
            <Segmented<SourceKind>
              block
              vertical={!screens.sm}
              data-testid="plan-kind-picker"
              options={SOURCE_KINDS.map((value) => ({
                value,
                icon: KIND_ICONS[value],
                label: <span data-testid={`plan-kind-option-${value}`}>{enumLabel(t, 'sourceKind', value)}</span>,
              }))}
            />
          </Form.Item>
          <Typography.Paragraph type="secondary" data-testid="plan-kind-hint">
            {t(`plans.kindHint.${kind}`)}
          </Typography.Paragraph>

          <SourceFields form={form} kind={kind} secretStored={hasStoredSecret(plan.data, kind)} onFilled={edited} />

          <Form.Item>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Button icon={<ApiOutlined />} onClick={onTest} loading={test.isPending} data-testid="plan-test-button">
                {t('plans.test.button')}
              </Button>
              {testResult && <SourceTestResult result={testResult} />}
            </Space>
          </Form.Item>

          {section(t('plans.sections.when'))}
          <ScheduleRows rows={values.schedules ?? []} />

          {section(t('plans.sections.where'))}
          {storages.data && storages.data.data.length === 0 ? (
            <Alert
              type="info"
              showIcon
              data-testid="plan-no-storages"
              message={t('plans.where.none')}
              action={
                <Button size="small" icon={<PlusOutlined />} onClick={() => navigate('/storages/new')} data-testid="plan-add-storage-button">
                  {t('plans.where.add')}
                </Button>
              }
              style={{ marginBottom: token.marginMD }}
            />
          ) : (
            <Form.Item name="storage_ids" label={t('plans.where.field')} rules={[{ required: true, type: 'array', min: 1, message: t('plans.where.required') }]}>
              <Select
                mode="multiple"
                data-testid="plan-storages-select"
                placeholder={t('plans.where.placeholder')}
                loading={storages.isLoading}
                options={storageOptions}
                optionFilterProp="title"
              />
            </Form.Item>
          )}

          {section(t('plans.sections.keep'))}
          <RetentionFields values={values} />

          <Divider />
          <Form.Item name="enabled" valuePropName="checked" label={t('plans.enabled')} extra={t('plans.enabledHint')}>
            <Switch data-testid="plan-enabled-switch" />
          </Form.Item>

          <Space wrap>
            <Button data-testid="plan-save-button" type="primary" htmlType="submit" loading={save.isPending}>
              {t('common.save')}
            </Button>
            {id != null && (
              <Popconfirm
                title={t('plans.deleteConfirm')}
                description={t('plans.deleteHint')}
                onConfirm={onDelete}
                okText={t('common.yes')}
                cancelText={t('common.no')}
                okButtonProps={{ danger: true, 'data-testid': 'plan-editor-delete-confirm' }}
              >
                <Button data-testid="plan-editor-delete-button" danger loading={remove.isPending}>
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
