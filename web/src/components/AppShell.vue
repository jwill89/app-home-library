<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import {
  faBarcodeRead,
  faBookArrowRight,
  faBooks,
  faGear,
  faGraduationCap,
  faPlus,
} from '@fortawesome/pro-regular-svg-icons'
import { useSession } from '@/app/session'

const session = useSession()

// Six places, in the bar across the top on a wide screen and along the bottom, under the
// thumb, on a phone or a tablet. A visitor has the library alone, and it fits in the bar on
// any screen.
// Each place is current for the pages under it: the Library for its books and papers.
const places = [
  { to: '/', label: 'Library', icon: faBooks, under: ['/books', '/papers'] },
  { to: '/scan', label: 'Scan', icon: faBarcodeRead, under: [] },
  { to: '/add', label: 'Add', icon: faPlus, under: [] },
  { to: '/loans', label: 'Loans', icon: faBookArrowRight, under: [] },
  { to: '/courses', label: 'Courses', icon: faGraduationCap, under: [] },
  { to: '/settings', label: 'Settings', icon: faGear, under: [] },
]
const shown = computed(() => (session.isVisitor ? places.slice(0, 1) : places))
const route = useRoute()
const current = (p: (typeof places)[number]) =>
  p.to === '/'
    ? route.path === '/' || p.under.some((u) => route.path.startsWith(u))
    : route.path.startsWith(p.to)
</script>

<template>
  <div class="shell">
    <a href="#main" class="shell__skip">Skip to the content</a>
    <header class="shell__bar">
      <RouterLink to="/" class="shell__brand">
        <img src="/icons/icon.svg" alt="" class="size-7" />
        <span>Home Library</span>
      </RouterLink>
      <nav
        class="shell__top-nav"
        :class="{ 'shell__top-nav--always': session.isVisitor }"
        aria-label="Main"
      >
        <RouterLink
          v-for="p in shown"
          :key="p.to"
          :to="p.to"
          class="shell__link"
          :class="{ 'shell__link--active': current(p) }"
          :aria-current="current(p) ? 'page' : undefined"
        >
          <FontAwesomeIcon :icon="p.icon" />
          {{ p.label }}
        </RouterLink>
      </nav>
    </header>
    <p v-if="session.isVisitor" class="shell__shared">
      {{ session.name ? `${session.name} shared this library with you` : 'A shared library' }}
      to browse. Nothing here can be changed.
    </p>
    <main
      id="main"
      tabindex="-1"
      class="shell__main focus:outline-none"
      :class="{ 'shell__main--no-tabs': session.isVisitor }"
    >
      <RouterView />
    </main>
    <nav v-if="!session.isVisitor" class="shell__tabs" aria-label="Main">
      <RouterLink
        v-for="p in places"
        :key="p.to"
        :to="p.to"
        class="shell__tab"
        :class="{ 'shell__tab--active': current(p) }"
        :aria-current="current(p) ? 'page' : undefined"
      >
        <FontAwesomeIcon :icon="p.icon" class="text-xl" />
        <span>{{ p.label }}</span>
      </RouterLink>
    </nav>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100dvh;
}

.shell__bar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  height: 3.5rem;
  padding: 0 1rem;
  background: var(--color-surface-panel);
  border-bottom: 1px solid var(--color-hairline);
}

.shell__brand {
  display: flex;
  align-items: center;
  min-height: var(--spacing-target);
  gap: 0.6rem;
  font-weight: 650;
  font-size: 1.125rem;
  color: var(--color-ink);
  text-decoration: none;
}

.shell__top-nav {
  display: none;
  gap: 0.25rem;
}

.shell__link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 2.75rem;
  padding: 0 0.9rem;
  border-radius: var(--radius-control);
  color: var(--color-ink-muted);
  text-decoration: none;
}

.shell__top-nav--always {
  display: flex;
}

.shell__shared {
  padding: 0.6rem 1rem;
  font-size: 0.875rem;
  line-height: 1.5;
  color: var(--color-ink-muted);
  background: var(--color-surface-page);
  border-bottom: 1px solid var(--color-hairline);
}

.shell__link:hover {
  color: var(--color-ink);
  background: var(--color-surface-page);
}

.shell__link--active {
  color: var(--color-accent);
  background: var(--color-highlight);
}

/* Off screen until a keyboard reaches it, first on every page. */
.shell__skip {
  position: absolute;
  top: 0.5rem;
  left: 0.5rem;
  z-index: 20;
  padding: 0.75rem 1rem;
  border-radius: var(--radius-control);
  background: var(--color-accent);
  color: var(--color-accent-contrast);
  font-weight: 600;
  transform: translateY(-200%);
}

.shell__skip:focus {
  transform: none;
}

.shell__main {
  padding: 1rem 1rem 6rem;
}

.shell__main--no-tabs {
  padding-bottom: 2rem;
}

.shell__tabs {
  position: fixed;
  inset: auto 0 0;
  z-index: 10;
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  padding-bottom: env(safe-area-inset-bottom);
  background: var(--color-surface-panel);
  border-top: 1px solid var(--color-hairline);
}

.shell__tab {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.2rem;
  min-height: 3.75rem;
  font-size: 0.75rem;
  color: var(--color-ink-muted);
  text-decoration: none;
}

.shell__tab--active {
  color: var(--color-accent);
}

@media (width >= 48rem) {
  .shell__main {
    padding: 1.5rem 1.5rem 6rem;
  }

  .shell__main--no-tabs {
    padding-bottom: 3rem;
  }
}

/* Six places with their names need about 54rem beside the brand. */
@media (width >= 64rem) {
  .shell__top-nav {
    display: flex;
  }

  .shell__tabs {
    display: none;
  }

  .shell__main {
    padding-bottom: 3rem;
  }
}
</style>
