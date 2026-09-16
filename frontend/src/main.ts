import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { useAuth } from './composables/useAuth'
import './index.css'

/**
 * Rehydrate auth state from the session cookie before mounting.
 * `isLoggedIn` is an in-memory singleton that resets on every full page
 * reload — without this, the router guard always sees `isLoggedIn=false`
 * right after a refresh, even with a valid session cookie still on disk.
 */
async function bootstrap(): Promise<void> {
  await useAuth().checkSession()

  const app = createApp(App)
  app.use(router)
  app.mount('#app')
}

bootstrap()
