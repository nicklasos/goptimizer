import { Button, Input, type FormInstance } from 'antd'
import { CopyOutlined, ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import message from '@/shared/utils/message'
import { copyToClipboard, generatePassword } from '@/shared/utils/password'

interface Props {
  form: FormInstance
  name?: string
  testId: string
  generateTestId: string
  copyTestId: string
  value?: string
  onChange?: React.ChangeEventHandler<HTMLInputElement>
}

export function PasswordInput({ form, name = 'password', testId, generateTestId, copyTestId, value, onChange }: Props) {
  const { t } = useTranslation()

  const addon = (
    <div style={{ display: 'flex', gap: 4 }}>
      <Button
        size="small"
        type="text"
        icon={<ReloadOutlined />}
        data-testid={generateTestId}
        onClick={() => form.setFieldValue(name, generatePassword())}
      />
      <Button
        size="small"
        type="text"
        icon={<CopyOutlined />}
        data-testid={copyTestId}
        onClick={async () => {
          await copyToClipboard(form.getFieldValue(name) || '')
          message.success(t('common.copied'))
        }}
      />
    </div>
  )

  return <Input.Password data-testid={testId} value={value} onChange={onChange} addonAfter={addon} />
}
