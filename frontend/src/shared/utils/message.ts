import { message } from 'antd'
import type { ReactNode } from 'react'

type Content = ReactNode

const customMessage = {
  success: (content: Content, duration = 4) => message.success(content, duration),
  error: (content: Content, duration = 9) => message.error(content, duration),
  warning: (content: Content, duration = 9) => message.warning(content, duration),
  info: (content: Content, duration = 4) => message.info(content, duration),
  loading: (content: Content, duration = 0) => message.loading(content, duration),
}

export default customMessage
