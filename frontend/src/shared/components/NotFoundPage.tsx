import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageError } from './PageState'

/** The page for a URL that matches no route. */
export default function NotFoundPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  return <PageError error={null} back={{ label: t('common.backHome'), onClick: () => navigate('/'), testId: 'not-found-home-button' }} />
}
