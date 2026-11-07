<script setup lang="ts">
import { computed } from 'vue'
import {
  faBookOpenReader,
  faBooks,
  faFileLines,
  faGift,
  faLayerGroup,
} from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import type { Sees } from '@/api/client'
import { useSession } from '@/app/session'

// The library's books, the ones wanted, its series, its reading, and the papers, under the
// one Library place in the navigation: those a visitor's link has anything for.
const session = useSession()
const all: {
  to: string
  label: string
  icon: typeof faBooks
  exact: boolean
  needs: keyof Sees
}[] = [
  { to: '/', label: 'Books', icon: faBooks, exact: true, needs: 'books' },
  { to: '/wishlist', label: 'Wishlist', icon: faGift, exact: false, needs: 'wishlist' },
  { to: '/series', label: 'Series', icon: faLayerGroup, exact: false, needs: 'books' },
  { to: '/reading', label: 'Reading', icon: faBookOpenReader, exact: false, needs: 'books' },
  { to: '/papers', label: 'Papers', icon: faFileLines, exact: false, needs: 'papers' },
]
const tabs = computed(() => all.filter((t) => session.sees[t.needs]))
</script>

<template>
  <!-- A link of papers alone has one page, and no tabs to choose between. -->
  <nav v-if="tabs.length > 1" aria-label="Library" class="tabs">
    <RouterLink
      v-for="t in tabs"
      :key="t.to"
      :to="t.to"
      class="tabs__link"
      :exact-active-class="t.exact ? 'tabs__link--active' : ''"
      :active-class="t.exact ? '' : 'tabs__link--active'"
      ><FontAwesomeIcon :icon="t.icon" /> {{ t.label }}</RouterLink
    >
  </nav>
</template>

<style scoped>
/* On a phone the tabs wrap onto a second row rather than run past the screen. */
.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0 0.25rem;
  margin-bottom: 1rem;
  border-bottom: 1px solid var(--color-hairline);
}

.tabs__link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: var(--spacing-target);
  padding: 0 1rem;
  margin-bottom: -1px;
  border-bottom: 3px solid transparent;
  color: var(--color-ink-muted);
  text-decoration: none;
}

.tabs__link:hover {
  color: var(--color-ink);
}

/* The current tab is said by its bar and its weight, not by its color alone. */
.tabs__link--active {
  border-bottom-color: var(--color-accent);
  color: var(--color-ink);
  font-weight: 600;
}
</style>
