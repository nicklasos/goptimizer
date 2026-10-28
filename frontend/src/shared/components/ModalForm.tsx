import type { ReactNode } from 'react'
import { Button, Form, Grid, Modal, Space, type FormInstance } from 'antd'
import { useTranslation } from 'react-i18next'

interface Props<V> {
  testId: string
  buttonPrefix: string
  title: ReactNode
  open: boolean
  onCancel: () => void
  form: FormInstance<V>
  onFinish: (values: V) => void
  submitting?: boolean
  submitText: ReactNode
  width?: number
  children: ReactNode
}

export function ModalForm<V>({
  testId,
  buttonPrefix,
  title,
  open,
  onCancel,
  form,
  onFinish,
  submitting,
  submitText,
  width = 520,
  children,
}: Props<V>) {
  const { t } = useTranslation()
  const screens = Grid.useBreakpoint()

  return (
    <Modal data-testid={testId} title={title} open={open} onCancel={onCancel} footer={null} width={screens.lg ? width : '100%'} forceRender>
      <Form form={form} onFinish={onFinish} layout="vertical" disabled={submitting}>
        {children}
        <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
          <Space>
            <Button data-testid={`${buttonPrefix}-cancel-button`} onClick={onCancel}>
              {t('common.cancel')}
            </Button>
            <Button data-testid={`${buttonPrefix}-submit-button`} type="primary" htmlType="submit" loading={submitting}>
              {submitText}
            </Button>
          </Space>
        </Form.Item>
      </Form>
    </Modal>
  )
}
