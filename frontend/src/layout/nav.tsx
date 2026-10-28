import { CloudServerOutlined, DashboardOutlined, TeamOutlined, UserOutlined } from '@ant-design/icons'
import { ROLE_ADMIN } from '@/auth/roles'
import type { NavItem } from './navMatch'

/** The top bar. An item is shown to the listed roles; a super admin sees all of them. The dashboard is the list of backup plans. */
export const TOP_NAV: NavItem[] = [
  {
    key: 'dashboard',
    labelKey: 'navigation.dashboard',
    icon: <DashboardOutlined />,
    testId: 'nav-dashboard',
    roles: 'all',
    path: '/',
    alsoPaths: ['/plans'],
  },
  {
    key: 'storages',
    labelKey: 'navigation.storages',
    icon: <CloudServerOutlined />,
    testId: 'nav-storages',
    roles: [ROLE_ADMIN],
    path: '/storages',
  },
]

/** The menu under the user's name. Signing out is added by the shell, after these. */
export const USER_MENU: NavItem[] = [
  {
    key: 'profile',
    labelKey: 'navigation.profile',
    icon: <UserOutlined />,
    testId: 'user-menu-profile',
    roles: 'all',
    path: '/profile',
  },
  {
    key: 'users',
    labelKey: 'navigation.users',
    icon: <TeamOutlined />,
    testId: 'user-menu-users',
    roles: [ROLE_ADMIN],
    path: '/users',
  },
]
