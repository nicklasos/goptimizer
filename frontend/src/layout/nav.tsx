import { FolderOutlined, SettingOutlined, TeamOutlined, UserOutlined } from '@ant-design/icons'
import type { NavItem } from './navMatch'

/** The top bar. Every signed-in user sees an item unless it is `adminOnly`. */
export const TOP_NAV: NavItem[] = [
  {
    key: 'dashboard',
    labelKey: 'navigation.dashboard',
    icon: <FolderOutlined />,
    testId: 'nav-dashboard',
    path: '/',
  },
  {
    key: 'settings',
    labelKey: 'navigation.settings',
    icon: <SettingOutlined />,
    testId: 'nav-settings',
    path: '/settings',
  },
]

/** The menu under the user's name. Signing out is added by the shell, after these. */
export const USER_MENU: NavItem[] = [
  {
    key: 'profile',
    labelKey: 'navigation.profile',
    icon: <UserOutlined />,
    testId: 'user-menu-profile',
    path: '/profile',
  },
  {
    key: 'users',
    labelKey: 'navigation.users',
    icon: <TeamOutlined />,
    testId: 'user-menu-users',
    adminOnly: true,
    path: '/users',
  },
]
