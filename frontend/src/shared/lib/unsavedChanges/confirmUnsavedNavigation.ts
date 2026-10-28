import { Modal, type ModalFuncProps } from 'antd'

type Translate = (key: string) => string
type ModalConfirm = (props: ModalFuncProps) => unknown

export function confirmUnsavedNavigation(
  isDirty: () => boolean,
  t: Translate,
  onConfirm: () => void,
  modalConfirm: ModalConfirm = Modal.confirm
): void {
  if (!isDirty()) {
    onConfirm()
    return
  }

  modalConfirm({
    title: t('messages.unsavedChangesTitle'),
    content: t('messages.unsavedChanges'),
    okText: t('messages.unsavedChangesLeave'),
    cancelText: t('common.cancel'),
    okButtonProps: { 'data-testid': 'unsaved-changes-leave-button' } as ModalFuncProps['okButtonProps'],
    cancelButtonProps: { 'data-testid': 'unsaved-changes-stay-button' } as ModalFuncProps['cancelButtonProps'],
    onOk: onConfirm,
  })
}
