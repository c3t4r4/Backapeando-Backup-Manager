/**
 * Vue Router 4 configuration with route guards
 *
 * Routes:
 * - `/` — redirects based on auth state
 * - `/login` — public route
 * - `/dashboard` — protected route (analytics overview)
 * - `/servers` — protected route (server management, moved out of /dashboard)
 * - `/history` — protected route
 * - `/settings` — protected route
 * - `/servers/new` — protected route
 * - `/servers/:id/edit` — protected route
 * - `/storage-targets` — protected route
 * - `/storage-targets/new` — protected route
 * - `/storage-targets/:id/edit` — protected route
 * - `/admin-users` — protected route
 * - `/admin-users/new` — protected route
 * - `/admin-users/:id/edit` — protected route
 *
 * Guard Logic:
 * - Unauthenticated users trying to access protected routes are redirected to `/login`
 * - Authenticated users visiting `/login` are redirected to `/dashboard`
 * - Logout clears state and allows re-access to `/login`
 */

import {
  createRouter,
  createWebHistory,
  type RouteRecordRaw,
  type NavigationGuardNext,
  type RouteLocationNormalized,
} from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'
import ServersView from '@/views/ServersView.vue'
import HistoryView from '@/views/HistoryView.vue'
import ServerNewView from '@/views/ServerNewView.vue'
import ServerEditView from '@/views/ServerEditView.vue'
import StorageTargetsView from '@/views/StorageTargetsView.vue'
import StorageTargetNewView from '@/views/StorageTargetNewView.vue'
import StorageTargetEditView from '@/views/StorageTargetEditView.vue'
import SettingsView from '@/views/SettingsView.vue'
import AdminUsersView from '@/views/AdminUsersView.vue'
import AdminUserNewView from '@/views/AdminUserNewView.vue'
import AdminUserEditView from '@/views/AdminUserEditView.vue'

/**
 * Augment Vue Router type definitions for custom meta
 * Allows `meta.requiresAuth` on route records
 */
// eslint-disable no-unused-vars
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
  }
}
// eslint-enable no-unused-vars

/**
 * Route definitions with meta information
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: () => {
      const { isLoggedIn } = useAuth()
      return isLoggedIn.value ? '/dashboard' : '/login'
    },
  },
  {
    path: '/login',
    component: LoginView,
    name: 'login',
    meta: {
      requiresAuth: false,
    },
  },
  {
    path: '/dashboard',
    component: DashboardView,
    name: 'dashboard',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/servers',
    component: ServersView,
    name: 'servers',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/history',
    component: HistoryView,
    name: 'history',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/servers/new',
    component: ServerNewView,
    name: 'server-new',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/servers/:id/edit',
    component: ServerEditView,
    name: 'server-edit',
    props: true,
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/storage-targets',
    component: StorageTargetsView,
    name: 'storage-targets',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/storage-targets/new',
    component: StorageTargetNewView,
    name: 'storage-target-new',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/storage-targets/:id/edit',
    component: StorageTargetEditView,
    name: 'storage-target-edit',
    props: true,
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/settings',
    component: SettingsView,
    name: 'settings',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/admin-users',
    component: AdminUsersView,
    name: 'admin-users',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/admin-users/new',
    component: AdminUserNewView,
    name: 'admin-user-new',
    meta: {
      requiresAuth: true,
    },
  },
  {
    path: '/admin-users/:id/edit',
    component: AdminUserEditView,
    name: 'admin-user-edit',
    props: true,
    meta: {
      requiresAuth: true,
    },
  },
]

/**
 * Create router instance with web history
 */
export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

/**
 * Before each route navigation guard
 *
 * Logic:
 * 1. If navigating to a protected route (requiresAuth=true) and not logged in → redirect to /login
 * 2. If navigating to /login and already logged in → redirect to /dashboard
 * 3. Otherwise, allow navigation
 *
 * Exported (not just registered inline) so `router.spec.ts` can import and
 * test this exact function instead of maintaining a second, hand-duplicated
 * copy of the guard logic that could silently drift from the real one.
 *
 * @param to — destination route
 * @param from — source route
 * @param next — navigation guard callback
 * @returns route location for redirection or true to allow navigation
 */
export function authGuard(
  to: RouteLocationNormalized,
  _from: RouteLocationNormalized,
  next: NavigationGuardNext
): void {
  const { isLoggedIn } = useAuth()
  const requiresAuth = to.meta.requiresAuth ?? false

  // Protected route: user must be logged in
  if (requiresAuth && !isLoggedIn.value) {
    next({ name: 'login' })
    return
  }

  // Login route: already logged in, redirect to dashboard
  if (to.path === '/login' && isLoggedIn.value) {
    next({ name: 'dashboard' })
    return
  }

  // Allow navigation
  next()
}

router.beforeEach(authGuard)

export default router
