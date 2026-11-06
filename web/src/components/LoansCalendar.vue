<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from 'primevue/button'
import { faCalendarPlus, faCopy } from '@fortawesome/pro-regular-svg-icons'
import { api, type CalendarLink } from '@/api/client'
import { copyText } from '@/app/clipboard'
import { useConfirmAction } from '@/app/confirm'
import { day } from '@/app/format'
import { useNotice } from '@/app/problem'
import PagePanel from './PagePanel.vue'

// The loans' due dates as a calendar the owner's phone subscribes to: each copy still out with
// a due date is an event that day, with a reminder that morning. The link is secret, shown
// once, and can be replaced or turned off.
const notice = useNotice()
const ask = useConfirmAction()
const link = ref<CalendarLink | null>(null)
const made = ref('')

async function load() {
  const { data, error } = await api.GET('/calendar-link')
  if (error) return notice.failed(error)
  link.value = data
}
onMounted(load)

async function make() {
  const { data, error } = await api.POST('/calendar-link')
  if (error) return notice.failed(error)
  made.value = `${location.origin}/calendar/${data.token}.ics`
  await load()
}

function replace() {
  ask(
    {
      header: 'Make a new calendar link?',
      message:
        'The link you have stops working, and a calendar subscribed to it gets no more due dates. Subscribe to the new one instead.',
      accept: 'Make a new link',
      danger: true,
    },
    make,
  )
}

function turnOff() {
  ask(
    {
      header: 'Turn the calendar link off?',
      message: 'A calendar subscribed to it gets no more due dates. You can make a new link later.',
      accept: 'Turn it off',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/calendar-link')
      if (error) return notice.failed(error)
      made.value = ''
      await load()
    },
  )
}

async function copy() {
  if (await copyText(made.value)) notice.done('Link copied')
  else notice.failed(new Error('The link could not be copied. Select it and copy it by hand.'))
}
</script>

<template>
  <PagePanel title="Due dates in your calendar">
    <p class="prose-text mb-4 text-sm text-ink-muted">
      A private link your phone's calendar can subscribe to. Each copy lent with a due date shows
      that day, with a reminder in the morning, and it goes when the copy comes back. In Google
      Calendar, add it under Other calendars, From URL; on an iPhone, in Settings, Calendar,
      Accounts, Add Subscribed Calendar.
    </p>
    <div v-if="made" class="mb-4" role="status">
      <p class="prose-text text-sm font-semibold">
        Your calendar link, shown this once: copy it into your calendar now.
      </p>
      <p class="mt-2 font-mono text-sm break-all">{{ made }}</p>
      <Button type="button" class="mt-2" @click="copy"
        ><FontAwesomeIcon :icon="faCopy" /> Copy the link</Button
      >
    </div>
    <p v-else-if="link?.made" class="mb-4 text-sm">
      A calendar link is on, made {{ day(link.createdAt) }}; last fetched
      {{ day(link.lastUsedAt).toLowerCase() }}.
    </p>
    <div class="flex flex-wrap gap-2">
      <Button v-if="!link?.made" type="button" @click="make"
        ><FontAwesomeIcon :icon="faCalendarPlus" /> Make a calendar link</Button
      >
      <template v-else>
        <Button type="button" severity="secondary" label="Make a new link" @click="replace" />
        <Button type="button" text severity="danger" label="Turn it off" @click="turnOff" />
      </template>
    </div>
  </PagePanel>
</template>
