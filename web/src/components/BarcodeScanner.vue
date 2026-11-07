<script setup lang="ts">
import { onMounted, onUnmounted, ref, useTemplateRef } from 'vue'
import Button from 'primevue/button'
import {
  faCamera,
  faCircleCheck,
  faCircleExclamation,
  faCircleInfo,
  faFlashlight,
  faStop,
} from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { cameraProblem, createDetector, type Detector, type ScanNote } from '@/app/barcodes'

// The one camera scanner. It reads barcodes until stopped and hands each new one to the page;
// the same code is not handed over twice within a few seconds, the time a book is held still.
// What the page made of the last one shows across the top of the picture, where the eyes are
// while scanning: a result further down the page goes unseen.
defineProps<{ note?: ScanNote | null }>()
const emit = defineEmits<{ code: [value: string] }>()

const icons = { good: faCircleCheck, info: faCircleInfo, bad: faCircleExclamation }

const video = useTemplateRef<HTMLVideoElement>('video')
const running = ref(false)
const starting = ref(false)
const problem = ref('')
const torch = ref<boolean | null>(null) // null: the camera has no light
const flash = ref(false)

let stream: MediaStream | null = null
let detector: Detector | null = null
let last = { value: '', at: 0 }

async function start() {
  problem.value = ''
  starting.value = true
  try {
    detector ??= await createDetector()
    stream = await navigator.mediaDevices.getUserMedia({
      video: {
        facingMode: { ideal: 'environment' },
        width: { ideal: 1920 },
        height: { ideal: 1080 },
      },
      audio: false,
    })
    const track = stream.getVideoTracks()[0]
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- an older browser's track has no getCapabilities
    const can = (track?.getCapabilities?.() ?? {}) as { torch?: boolean; focusMode?: string[] }
    torch.value = can.torch ? false : null
    if (can.focusMode?.includes('continuous')) {
      await track?.applyConstraints({
        advanced: [{ focusMode: 'continuous' } as MediaTrackConstraintSet],
      })
    }
    if (video.value) {
      video.value.srcObject = stream
      await video.value.play()
    }
    running.value = true
    void tick()
  } catch (e) {
    problem.value = cameraProblem(e)
    stop()
  } finally {
    starting.value = false
  }
}

function stop() {
  running.value = false
  stream?.getTracks().forEach((t) => t.stop())
  stream = null
  torch.value = null
  if (video.value) {
    video.value.srcObject = null
  }
}

async function tick() {
  const el = video.value
  if (!running.value || !el || !detector) {
    return
  }
  if (el.readyState >= 2) {
    try {
      for (const found of await detector.detect(el)) {
        heard(found.rawValue)
      }
    } catch {
      // A frame that cannot be read is skipped; the next one is a moment away.
    }
  }
  setTimeout(() => void tick(), 120)
}

function heard(value: string) {
  const now = Date.now()
  if (!value || (value === last.value && now - last.at < 3000)) {
    return
  }
  last = { value, at: now }
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- Safari has no vibrate, whatever the DOM types say
  navigator.vibrate?.(60)
  flash.value = true
  setTimeout(() => (flash.value = false), 300)
  emit('code', value)
}

async function toggleTorch() {
  const track = stream?.getVideoTracks()[0]
  if (!track || torch.value === null) {
    return
  }
  const on = !torch.value
  await track.applyConstraints({ advanced: [{ torch: on } as MediaTrackConstraintSet] })
  torch.value = on
}

// A page in the background has no use for the camera, and the battery does.
function hidden() {
  if (document.hidden) {
    stop()
  }
}
onMounted(() => document.addEventListener('visibilitychange', hidden))
onUnmounted(() => {
  document.removeEventListener('visibilitychange', hidden)
  stop()
})

defineExpose({ start, stop })
</script>

<template>
  <div class="flex flex-col gap-3">
    <div v-show="running" class="scanner">
      <video ref="video" class="scanner__video" playsinline muted aria-label="Camera view" />
      <div class="scanner__target" :class="{ 'scanner__target--hit': flash }" aria-hidden="true" />
      <!-- Said aloud by the page's own status line; here it is for the eyes. -->
      <p
        v-if="note"
        :key="note.at"
        class="scanner__note"
        :class="`scanner__note--${note.tone}`"
        aria-hidden="true"
      >
        <FontAwesomeIcon :icon="icons[note.tone]" class="scanner__icon" />
        <span>{{ note.text }}</span>
      </p>
    </div>
    <div class="flex flex-wrap gap-2">
      <Button v-if="!running" type="button" :loading="starting" @click="start">
        <FontAwesomeIcon :icon="faCamera" /> Start the camera
      </Button>
      <Button v-else type="button" severity="secondary" @click="stop">
        <FontAwesomeIcon :icon="faStop" /> Stop the camera
      </Button>
      <Button
        v-if="running && torch !== null"
        type="button"
        severity="secondary"
        :aria-pressed="torch"
        @click="toggleTorch"
      >
        <FontAwesomeIcon :icon="faFlashlight" /> {{ torch ? 'Light off' : 'Light on' }}
      </Button>
    </div>
    <p v-if="problem" class="text-status-danger" role="alert">{{ problem }}</p>
    <p v-if="running" class="text-sm text-ink-muted">
      Hold the barcode inside the box, a hand's width from the camera.
    </p>
  </div>
</template>

<style scoped>
.scanner {
  position: relative;
  overflow: hidden;
  max-width: 40rem;
  aspect-ratio: 4 / 3;
  border-radius: var(--radius-control);
  background: var(--color-shade);
}

.scanner__video {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* Where to hold the barcode: wide and low, the shape of the one on a book's back. */
.scanner__target {
  position: absolute;
  inset: 32% 12%;
  border: 3px solid var(--color-on-shade);
  border-radius: var(--radius-control);
  box-shadow: 0 0 0 100vmax var(--color-shade);
  transition: border-color 0.15s;
}

.scanner__target--hit {
  border-color: var(--color-accent);
}

.scanner__note {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  left: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin: 0;
  padding: 0.6rem 0.8rem;
  border-left: 6px solid var(--scanner-tone);
  border-radius: var(--radius-control);
  background: var(--color-surface-panel);
  color: var(--color-ink);
  font-weight: 600;
  animation: scanner-arrive 0.25s ease-out;
}

.scanner__icon {
  flex: none;
  font-size: 1.25rem;
  color: var(--scanner-tone);
}

.scanner__note--good {
  --scanner-tone: var(--color-status-success);
}

.scanner__note--info {
  --scanner-tone: var(--color-accent);
}

.scanner__note--bad {
  --scanner-tone: var(--color-status-danger);
}

/* A new result drops in, so a second scan's result is seen to be new. */
@keyframes scanner-arrive {
  from {
    opacity: 0;
    transform: translateY(-0.5rem);
  }
}
</style>
