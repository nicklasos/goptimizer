import { Suspense, useMemo, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { Button, Drawer, Dropdown, Grid, Layout, Menu, type MenuProps } from 'antd'
import { DownOutlined, LogoutOutlined, MenuOutlined, UserOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/auth/authStore'
import { CrashScreen } from '@/shared/components/CrashScreen'
import { ErrorBoundary } from '@/shared/components/ErrorBoundary'
import { PageLoading } from '@/shared/components/PageState'
import { useGuardedNavigate } from '@/shared/lib/unsavedChanges'
import { TOP_NAV, USER_MENU } from './nav'
import { selectNavKey, visibleNav, type NavItem } from './navMatch'
import { TopProgressBar } from './TopProgressBar'
import './AppShell.css'

const { Header, Content } = Layout
const LOGOUT_KEY = 'logout'

export function AppShell() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useGuardedNavigate()
  const isMobile = !Grid.useBreakpoint().md
  const [drawerOpen, setDrawerOpen] = useState(false)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)

  const isAdmin = user?.is_admin === true

  const topItems = useMemo(() => visibleNav(TOP_NAV, isAdmin), [isAdmin])
  const userItems = useMemo(() => visibleNav(USER_MENU, isAdmin), [isAdmin])

  const toMenuItems = (items: NavItem[]) =>
    items.map((item) => ({
      key: item.key,
      icon: item.icon,
      label: <span data-testid={item.testId}>{t(item.labelKey)}</span>,
    }))

  const go = (items: NavItem[], key: string) => {
    const item = items.find((entry) => entry.key === key)
    if (item) navigate(item.path)
  }

  const topSelected = selectNavKey(topItems, location.pathname)
  const userSelected = selectNavKey(userItems, location.pathname)

  const nav = (
    <Menu
      data-testid="main-menu"
      mode={isMobile ? 'inline' : 'horizontal'}
      className="top-nav"
      selectedKeys={topSelected ? [topSelected] : []}
      items={toMenuItems(topItems)}
      onClick={({ key }) => {
        go(topItems, key)
        setDrawerOpen(false)
      }}
    />
  )

  const userMenu: MenuProps = {
    selectedKeys: userSelected ? [userSelected] : [],
    items: [
      ...toMenuItems(userItems),
      { type: 'divider' },
      {
        key: LOGOUT_KEY,
        icon: <LogoutOutlined />,
        label: <span data-testid="user-menu-logout">{t('auth.logout')}</span>,
      },
    ],
    onClick: ({ key }) => (key === LOGOUT_KEY ? logout() : go(userItems, key)),
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <TopProgressBar />
      <Header className="site-layout-header" data-testid="app-header">
        {isMobile && (
          <Button data-testid="menu-toggle-button" type="text" icon={<MenuOutlined />} onClick={() => setDrawerOpen(true)} />
        )}
        <a
          href="/"
          className="app-logo"
          data-testid="app-logo"
          aria-label={t('navigation.dashboard')}
          onClick={(event) => {
            event.preventDefault()
            navigate('/')
          }}
        >
          {isMobile ? (
            <img src="/logo.svg" alt="" width={32} height={32} />
          ) : (
            <img src="/logo-wordmark.svg" alt="" width={132} height={36} />
          )}
        </a>
        {isMobile ? <div className="top-nav-spacer" /> : nav}
        <Dropdown menu={userMenu} placement="bottomRight" trigger={['click']}>
          <Button data-testid="user-menu-button" type="text" icon={<UserOutlined />} className="user-menu-button">
            {!isMobile && (
              <>
                <span data-testid="user-menu-name" className="user-menu-name">
                  {user?.name || user?.email}
                </span>
                <DownOutlined className="user-menu-caret" />
              </>
            )}
          </Button>
        </Dropdown>
      </Header>

      {isMobile && (
        <Drawer
          data-testid="mobile-menu-drawer"
          title={<img src="/logo.svg" alt="" width={28} height={28} />}
          placement="left"
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          styles={{ body: { padding: 0 } }}
          width={240}
        >
          {nav}
        </Drawer>
      )}

      <Content className="site-layout-content" data-testid="main-content">
        {/* Keyed by path: a page that crashed is replaced as soon as the user navigates elsewhere */}
        <ErrorBoundary key={location.pathname} fallback={(_, reset) => <CrashScreen onRetry={reset} />}>
          <Suspense fallback={<PageLoading />}>
            <Outlet />
          </Suspense>
        </ErrorBoundary>
      </Content>
    </Layout>
  )
}
