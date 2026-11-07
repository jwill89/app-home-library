import { computed, onMounted, ref } from 'vue'
import { api, type QueueCounts } from '@/api/client'
import { useFeed } from './feed'

/**
 * The background lookup's progress: a page that shows it calls this once. Every lookup queued,
 * tried, or done reaches the page on the change feed, and the counts are fetched again;
 * `onProgress` runs when one has finished (the page's list fetches again on it). The lookup is
 * the owner's; for a visitor (`enabled` false) nothing is asked and nothing shows.
 */
export function useQueue(onProgress?: () => unknown, enabled = true) {
  const queue = ref<QueueCounts>({ pending: 0, waiting: 0, done: 0, failed: 0 })
  const total = computed(
    () => queue.value.pending + queue.value.waiting + queue.value.done + queue.value.failed,
  )
  const progress = computed(() =>
    total.value ? Math.round(((queue.value.done + queue.value.failed) / total.value) * 100) : 100,
  )

  async function load() {
    const { data } = await api.GET('/enrichment')
    if (data) {
      queue.value = data
    }
  }

  useFeed(['lookup'], async () => {
    const finished = queue.value.done + queue.value.failed
    await load()
    if (queue.value.done + queue.value.failed !== finished) {
      await onProgress?.()
    }
  })

  onMounted(() => {
    if (enabled) {
      void load()
    }
  })

  return { queue, total, progress }
}
