import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { setFormatLanguage } from '@/shared/utils/format'
import { LANGUAGES } from '@/shared/utils/languages'

import enTranslation from '../locales/en/translation.json'
import ukTranslation from '../locales/uk/translation.json'

const resources = {
  en: { translation: enTranslation },
  uk: { translation: ukTranslation },
}

const savedLanguage = localStorage.getItem('i18nextLng')
const initialLanguage = LANGUAGES.find((language) => language.code === savedLanguage)?.code ?? 'en'

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    lng: initialLanguage,
    fallbackLng: 'en',
    debug: false,
    detection: {
      order: ['localStorage'],
      caches: ['localStorage'],
      lookupLocalStorage: 'i18nextLng',
    },
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
  })

function applyLanguage(lng: string) {
  setFormatLanguage(lng)
  document.documentElement.lang = lng
}

applyLanguage(i18n.language)
i18n.on('languageChanged', applyLanguage)

export default i18n
