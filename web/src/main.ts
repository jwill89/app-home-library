import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { config as fontAwesome } from '@fortawesome/fontawesome-svg-core'
import App from './App.vue'
import { cspNonce } from './app/csp'
import { router } from './app/router'
import { useSession } from './app/session'
import { darkClass, startScheme } from './app/scheme'
import { preset } from './app/theme/preset'
import '@fortawesome/fontawesome-svg-core/styles.css'
import './app/theme/tailwind.css'

// The icon CSS ships in the stylesheet above instead of being injected at runtime.
fontAwesome.autoAddCss = false

startScheme()

createApp(App)
  .use(createPinia())
  .use(router)
  .use(PrimeVue, {
    csp: { nonce: cspNonce },
    theme: {
      preset,
      // Light or dark as the device is set, or as the owner chose (app/scheme.ts).
      options: {
        darkModeSelector: `.${darkClass}`,
        cssLayer: { name: 'primevue', order: 'theme, base, primevue' },
      },
    },
  })
  .use(ToastService)
  .use(ConfirmationService)
  .component('FontAwesomeIcon', FontAwesomeIcon)
  .mount('#app')

// The service worker keeps the app's files for the installed app to open offline. The Scan
// page's are fetched now, for the owner, so it works offline without having been opened first.
if ('serviceWorker' in navigator) {
  void navigator.serviceWorker.register('/sw.js').catch(() => undefined)
  void router.isReady().then(() => {
    if (useSession().isOwner && navigator.onLine) void import('@/views/ScanView.vue')
  })
}
