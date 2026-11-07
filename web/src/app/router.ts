import { createRouter, createWebHistory } from 'vue-router'
import { api, type Sees } from '@/api/client'
import { useSession } from './session'

declare module 'vue-router' {
  interface RouteMeta {
    /** Open without signing in. */
    public?: boolean
    /** Open to a visitor who came by a share link, as well as the owner. */
    reader?: boolean
    /** What a visitor's link has to show for the page to be theirs. */
    shows?: keyof Sees
    title?: string
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/components/AppShell.vue'),
      children: [
        {
          path: '',
          component: () => import('@/views/LibraryView.vue'),
          meta: { title: 'Library', reader: true, shows: 'books' },
        },
        {
          path: 'reading',
          component: () => import('@/views/ReadingView.vue'),
          meta: { title: 'Reading', reader: true, shows: 'books' },
        },
        {
          path: 'series',
          component: () => import('@/views/SeriesListView.vue'),
          meta: { title: 'Series', reader: true, shows: 'books' },
        },
        {
          path: 'series/:id(\\d+)',
          component: () => import('@/views/SeriesView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
          meta: { reader: true, shows: 'books' },
        },
        {
          path: 'wishlist',
          component: () => import('@/views/WishlistView.vue'),
          meta: { title: 'Wishlist', reader: true, shows: 'wishlist' },
        },
        {
          path: 'papers',
          component: () => import('@/views/DocumentsView.vue'),
          meta: { title: 'Papers', reader: true, shows: 'papers' },
        },
        {
          path: 'papers/new',
          component: () => import('@/views/DocumentEditView.vue'),
          meta: { title: 'Add a paper' },
        },
        // Links and bookmarks from before documents were called papers (2026-11-05).
        {
          path: 'documents/:rest(.*)*',
          redirect: (to) => {
            const rest = ([] as string[]).concat(to.params.rest ?? []).join('/')
            return { path: rest ? `/papers/${rest}` : '/papers', query: to.query }
          },
        },
        {
          path: 'papers/:id(\\d+)',
          component: () => import('@/views/DocumentView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
          meta: { reader: true, shows: 'papers' },
        },
        {
          path: 'papers/:id(\\d+)/edit',
          component: () => import('@/views/DocumentEditView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
          meta: { title: 'Edit the paper' },
        },
        {
          path: 'scan',
          component: () => import('@/views/ScanView.vue'),
          meta: { title: 'Scan' },
        },
        {
          path: 'add',
          component: () => import('@/views/AddBookView.vue'),
          meta: { title: 'Add a book' },
        },
        {
          path: 'books/:id(\\d+)',
          component: () => import('@/views/BookView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
          meta: { reader: true, shows: 'books' },
        },
        {
          path: 'books/:id(\\d+)/edit',
          component: () => import('@/views/EditBookView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
          meta: { title: 'Edit book' },
        },
        {
          path: 'loans',
          component: () => import('@/views/LoansView.vue'),
          meta: { title: 'Loans' },
        },
        {
          path: 'courses',
          component: () => import('@/views/CoursesView.vue'),
          meta: { title: 'Courses' },
        },
        {
          path: 'courses/:id(\\d+)',
          component: () => import('@/views/CourseView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
        },
        {
          path: 'shelves',
          component: () => import('@/views/ShelvesView.vue'),
          meta: { title: 'Shelves' },
        },
        {
          path: 'shelves/:id(\\d+)/check',
          component: () => import('@/views/ShelfCheckView.vue'),
          props: (r) => ({ id: Number(r.params.id) }),
          meta: { title: 'Shelf check' },
        },
        {
          path: 'gaps',
          component: () => import('@/views/GapsView.vue'),
          meta: { title: 'Missing details' },
        },
        {
          path: 'suggestions',
          component: () => import('@/views/SuggestionsView.vue'),
          meta: { title: 'Suggested changes' },
        },
        {
          path: 'labels',
          component: () => import('@/views/LabelsView.vue'),
          meta: { title: 'Labels' },
        },
        {
          path: 'import',
          component: () => import('@/views/ImportView.vue'),
          meta: { title: 'Import' },
        },
        {
          path: 'settings',
          component: () => import('@/views/SettingsView.vue'),
          meta: { title: 'Settings' },
        },
      ],
    },
    {
      path: '/sign-in',
      component: () => import('@/views/SignInView.vue'),
      meta: { public: true, title: 'Sign in' },
    },
    {
      path: '/setup',
      component: () => import('@/views/SetupView.vue'),
      meta: { public: true, title: 'Set up this device' },
    },
    {
      path: '/link-ended',
      component: () => import('@/views/LinkEndedView.vue'),
      meta: { public: true, title: 'Link ended' },
    },
    { path: '/:rest(.*)*', redirect: '/' },
  ],
  // eslint-disable-next-line @typescript-eslint/no-deprecated -- vue-router 5's useScrollRestoration replaces it; moving is a change of its own
  scrollBehavior: (_to, _from, saved) => saved ?? { top: 0 },
})

router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.loaded) {
    await session.load()
  }
  if (to.meta.public) {
    return
  }
  if (session.role === 'none') {
    return { path: '/sign-in', query: to.fullPath === '/' ? {} : { next: to.fullPath } }
  }
  // A visitor reads what their link shows; everywhere else is the owner's.
  const theirs = to.meta.reader && (!to.meta.shows || session.sees[to.meta.shows])
  if (session.isVisitor && !theirs && to.path !== session.home) {
    return session.home
  }
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · Home Library` : 'Home Library'
})

// A link revoked or run out while a visitor browses, or a sign-in gone stale: the next answer
// from the server says so, and the page follows.
api.use({
  onResponse({ response }) {
    const session = useSession()
    if (response.status !== 401 || session.role === 'none') {
      return
    }
    const visitor = session.isVisitor
    session.forget()
    void router.replace(visitor ? '/link-ended' : '/sign-in')
  },
})
