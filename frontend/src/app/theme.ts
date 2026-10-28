import type { ThemeConfig } from 'antd'

/**
 * The one place for branding. Pages style themselves with AntD tokens (`theme.useToken()`),
 * never with literal colours, so changing a project's look is an edit here only.
 */
export const theme: ThemeConfig = {
  cssVar: true,
  hashed: false,
  token: {
    colorPrimary: '#1677ff',
    borderRadius: 6,
  },
  components: {
    Layout: {
      siderBg: '#ffffff',
      headerBg: '#ffffff',
      headerHeight: 56,
      headerPadding: '0 16px',
    },
    Menu: {
      activeBarBorderWidth: 0,
      itemBorderRadius: 6,
    },
  },
}
