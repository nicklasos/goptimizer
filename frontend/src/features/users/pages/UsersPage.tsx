import { useState } from 'react'
import { Button, Card, Form, Input, Switch, Tag, Tooltip } from 'antd'
import { KeyOutlined, PlusOutlined, TeamOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { useCurrentUser } from '@/auth/authStore'
import { ModalForm } from '@/shared/components/ModalForm'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageStack } from '@/shared/components/PageStack'
import { PasswordInput } from '@/shared/components/PasswordInput'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { RowActions } from '@/shared/components/RowActions'
import { usePageParams, useTablePagination } from '@/shared/hooks/usePageParams'
import { formatDateTime } from '@/shared/utils/format'
import message from '@/shared/utils/message'
import { generatePassword } from '@/shared/utils/password'
import { applyFormErrors } from '@/shared/utils/formErrors'
import { useDeleteUser, useSaveUser, useSetUserPassword, useUsers } from '../hooks'
import { canDeleteOrDemote } from '../policy'
import type { ManagedUser, UserFormValues } from '../types'

const FORM_FIELDS = ['name', 'email', 'password', 'is_admin'] as const
const KEY_FIELDS = { 'auth.user_exists': 'email', 'users.last_admin': 'is_admin' }

/** `/users`: every account, admins and users, in one table. */
export default function UsersPage() {
  const { t } = useTranslation()
  const currentUser = useCurrentUser()
  const [form] = Form.useForm<UserFormValues>()
  const [passwordForm] = Form.useForm<{ password: string }>()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<ManagedUser | null>(null)
  const [passwordTarget, setPasswordTarget] = useState<ManagedUser | null>(null)
  const paging = usePageParams()
  const users = useUsers(paging.params)
  const pagination = useTablePagination(paging, users.data)
  const save = useSaveUser()
  const setPassword = useSetUserPassword()
  const remove = useDeleteUser()

  const openModal = (record: ManagedUser | null) => {
    setEditing(record)
    form.resetFields()
    form.setFieldsValue(record ? { name: record.name, email: record.email, is_admin: record.is_admin } : { password: generatePassword(), is_admin: false })
    setModalOpen(true)
  }

  const closeModal = () => {
    setModalOpen(false)
    setEditing(null)
    form.resetFields()
  }

  const closePasswordModal = () => {
    setPasswordTarget(null)
    passwordForm.resetFields()
  }

  const onFinish = (values: UserFormValues) =>
    save.mutate(
      { id: editing?.id, values },
      {
        onSuccess: () => {
          message.success(editing ? t('userManagement.userUpdated') : t('userManagement.userCreated'))
          closeModal()
        },
        onError: (error) => {
          const unplaced = applyFormErrors(form, t, error, FORM_FIELDS, { keyFields: KEY_FIELDS })
          if (unplaced) message.error(unplaced)
        },
      }
    )

  const onPasswordFinish = ({ password }: { password: string }) => {
    if (!passwordTarget) return
    setPassword.mutate(
      { id: passwordTarget.id, password },
      {
        onSuccess: () => {
          message.success(t('userManagement.passwordUpdated'))
          closePasswordModal()
        },
      }
    )
  }

  const onDelete = (record: ManagedUser) => remove.mutate(record.id, { onSuccess: () => message.success(t('userManagement.userDeleted')) })

  const actions = (record: ManagedUser) => (
    <RowActions
      entity="user"
      id={record.id}
      onEdit={() => openModal(record)}
      onDelete={canDeleteOrDemote(currentUser, record) ? () => onDelete(record) : undefined}
      deleteTitle={t('userManagement.deleteConfirm')}
      deleteDescription={t('userManagement.deleteConfirmDescription')}
    >
      <Tooltip title={t('userManagement.setPassword')}>
        <Button
          type="text"
          icon={<KeyOutlined />}
          onClick={() => {
            setPasswordTarget(record)
            passwordForm.setFieldsValue({ password: generatePassword() })
          }}
          size="small"
          data-testid={`set-password-user-button-${record.id}`}
        />
      </Tooltip>
    </RowActions>
  )

  const columns: ColumnsType<ManagedUser> = [
    { title: t('userManagement.userName'), dataIndex: 'name', key: 'name', ellipsis: true },
    { title: t('userManagement.userEmail'), dataIndex: 'email', key: 'email' },
    { title: t('userManagement.admin'), dataIndex: 'is_admin', key: 'is_admin', width: 100, render: (isAdmin: boolean) => <AdminTag isAdmin={isAdmin} /> },
    {
      title: t('common.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (value: string) => formatDateTime(value),
    },
    { title: t('common.actions'), key: 'actions', width: 140, fixed: 'right', render: (_, record) => actions(record) },
  ]

  return (
    <PageStack testId="users-page">
      <Card>
        <PageHeader
          icon={<TeamOutlined />}
          title={t('navigation.users')}
          mainAction={
            <Button data-testid="add-user-button" type="primary" icon={<PlusOutlined />} onClick={() => openModal(null)}>
              {t('userManagement.add')}
            </Button>
          }
        />
        <ResponsiveTable
          testId="users-table"
          items={users.data?.data}
          loading={users.isFetching || remove.isPending}
          columns={columns}
          emptyText={t('userManagement.empty')}
          pagination={pagination}
          cardActions={actions}
          cardTitle={(item) => item.name}
          cardTestId={(item) => `user-card-${item.id}`}
          renderCard={(item) => (
            <>
              <div>{item.email}</div>
              <AdminTag isAdmin={item.is_admin} />
              <div>
                {t('common.createdAt')}: {formatDateTime(item.created_at)}
              </div>
            </>
          )}
        />
      </Card>

      <ModalForm
        testId="user-form-modal"
        buttonPrefix="user-modal"
        title={editing ? t('userManagement.edit') : t('userManagement.add')}
        open={modalOpen}
        onCancel={closeModal}
        form={form}
        onFinish={onFinish}
        submitting={save.isPending}
        submitText={editing ? t('common.save') : t('common.create')}
      >
        <Form.Item name="name" label={t('userManagement.userName')} rules={[{ required: true, whitespace: true, message: t('common.required') }]}>
          <Input data-testid="user-name-input" />
        </Form.Item>
        <Form.Item
          name="email"
          label={t('userManagement.userEmail')}
          rules={[
            { required: true, message: t('common.required') },
            { type: 'email', message: t('auth.emailInvalid') },
          ]}
        >
          <Input data-testid="user-email-input" />
        </Form.Item>
        {!editing && (
          <Form.Item
            name="password"
            label={t('userManagement.password')}
            rules={[
              { required: true, message: t('common.required') },
              { min: 8, message: t('userManagement.passwordTooShort') },
            ]}
          >
            <PasswordInput
              form={form}
              testId="user-password-input"
              generateTestId="generate-password-button"
              copyTestId="copy-password-button"
            />
          </Form.Item>
        )}
        <Form.Item name="is_admin" label={t('userManagement.admin')} valuePropName="checked" extra={t('userManagement.adminHint')}>
          <Switch data-testid="user-admin-switch" disabled={editing != null && !canDeleteOrDemote(currentUser, editing)} />
        </Form.Item>
      </ModalForm>

      <ModalForm
        testId="user-password-modal"
        buttonPrefix="password-modal"
        title={t('userManagement.setPassword')}
        open={passwordTarget != null}
        onCancel={closePasswordModal}
        form={passwordForm}
        onFinish={onPasswordFinish}
        submitting={setPassword.isPending}
        submitText={t('common.save')}
        width={480}
      >
        <Form.Item
          name="password"
          label={t('userManagement.newPassword')}
          rules={[
            { required: true, message: t('common.required') },
            { min: 8, message: t('userManagement.passwordTooShort') },
          ]}
        >
          <PasswordInput
            form={passwordForm}
            testId="user-new-password-input"
            generateTestId="generate-new-password-button"
            copyTestId="copy-new-password-button"
          />
        </Form.Item>
      </ModalForm>
    </PageStack>
  )
}

function AdminTag({ isAdmin }: { isAdmin: boolean }) {
  const { t } = useTranslation()
  if (!isAdmin) return null
  return (
    <Tag bordered={false} color="processing" data-testid="user-admin-tag">
      {t('userManagement.admin')}
    </Tag>
  )
}
