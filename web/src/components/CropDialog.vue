<script setup lang="ts">
import { computed, ref, useTemplateRef, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import { faArrowsRotate, faRotateRight } from '@fortawesome/pro-regular-svg-icons'
import { detectCover } from '@/app/detect'
import { outputSize, warp, type Point, type Quad } from '@/app/perspective'

// Straightens and crops a photo in the browser: the owner puts a corner on each of the thing's
// corners (a book's cover, a page), and what they mark becomes a flat rectangle no longer than
// `size` pixels on its long side. A phone's photo is many times that, and most of it is table.
const props = withDefaults(
  defineProps<{
    file: File | null
    /** What is photographed, as the instructions name it. */
    subject?: 'cover' | 'page'
    /** The long side of the result: a cover needs 1600, a page's small print more. */
    size?: number
  }>(),
  { subject: 'cover', size: 1600 },
)
const emit = defineEmits<{ done: [blob: Blob] }>()
const visible = defineModel<boolean>('visible', { required: true })

// Working at a little over the result's size keeps the straightening quick on a phone and
// still gives the result all the detail it can hold.
const working = computed(() => Math.round(props.size * 1.4))
const thing = computed(() => (props.subject === 'page' ? 'page' : 'book'))

const canvas = useTemplateRef<HTMLCanvasElement>('canvas')
const loupe = useTemplateRef<HTMLCanvasElement>('loupe')
const stage = useTemplateRef<HTMLDivElement>('stage')
const source = ref<HTMLCanvasElement | null>(null) // the photo, upright, at working size
const corners = ref<Quad>([
  { x: 0, y: 0 },
  { x: 0, y: 0 },
  { x: 0, y: 0 },
  { x: 0, y: 0 },
])
const dragging = ref(-1)
const busy = ref(false)
const failed = ref('')
// The corners the detection found, which Reset returns to, and what it said about them.
const found = ref<Quad | null>(null)
const finding = ref(false)
const note = ref('')

const names = ['Top-left corner', 'Top-right corner', 'Bottom-right corner', 'Bottom-left corner']

// Everything is laid out in the photo's own pixels and scaled by the browser to fit the
// screen: the canvas by CSS, the outline by its viewBox, the corners by percentages. Nothing
// is measured until a finger moves a corner, when the page is certainly laid out.
const width = computed(() => source.value?.width ?? 1)
const height = computed(() => source.value?.height ?? 1)
const at = (p: Point) => ({
  left: `${(p.x / width.value) * 100}%`,
  top: `${(p.y / height.value) * 100}%`,
})
const outline = computed(() => corners.value.map((p) => `${p.x},${p.y}`).join(' '))
// The photo outside the marked shape is shaded: the whole photo, less the shape.
const shade = computed(() => {
  const [a, b, c, d] = corners.value
  return `M0 0H${width.value}V${height.value}H0Z M${a.x} ${a.y}L${b.x} ${b.y}L${c.x} ${c.y}L${d.x} ${d.y}Z`
})

watch(
  () => [visible.value, props.file] as const,
  async ([open, file]) => {
    if (!open || !file) return
    failed.value = ''
    try {
      // The browser applies the photo's own orientation, which a phone records sideways.
      const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
      const fit = Math.min(1, working.value / Math.max(bitmap.width, bitmap.height))
      const c = document.createElement('canvas')
      c.width = Math.round(bitmap.width * fit)
      c.height = Math.round(bitmap.height * fit)
      c.getContext('2d')?.drawImage(bitmap, 0, 0, c.width, c.height)
      bitmap.close()
      source.value = c
      await find()
    } catch {
      failed.value = 'That file could not be read as an image.'
    }
  },
  { immediate: true },
)

// The photo is drawn once at its working size, when both it and the dialog's canvas exist.
watch(
  [source, canvas],
  ([src, view]) => {
    if (!src || !view) return
    view.width = src.width
    view.height = src.height
    view.getContext('2d')?.drawImage(src, 0, 0)
  },
  { flush: 'post' },
)

// The corners go where the detection found the cover, or a little in from the photo's edges,
// where a cover usually is, until the owner moves them.
async function find() {
  const src = source.value
  if (!src) return
  found.value = null
  reset()
  finding.value = true
  note.value = `Looking for the edges of the ${props.subject}…`
  const quad = await detectCover(src)
  finding.value = false
  if (source.value !== src) return // turned or replaced while it looked
  found.value = quad
  if (quad) corners.value = quad
  note.value = quad
    ? `The corners are on the ${props.subject} that was found. Move any that are off.`
    : `No clear edges were found. Drag the corners onto the ${props.subject}.`
}

function reset() {
  const src = source.value
  if (!src) return
  if (found.value) {
    corners.value = found.value
    return
  }
  const mx = src.width * 0.08
  const my = src.height * 0.08
  corners.value = [
    { x: mx, y: my },
    { x: src.width - mx, y: my },
    { x: src.width - mx, y: src.height - my },
    { x: mx, y: src.height - my },
  ]
}

function rotate() {
  const src = source.value
  if (!src) return
  const c = document.createElement('canvas')
  c.width = src.height
  c.height = src.width
  const ctx = c.getContext('2d')
  if (!ctx) return
  ctx.translate(c.width, 0)
  ctx.rotate(Math.PI / 2)
  ctx.drawImage(src, 0, 0)
  source.value = c
  void find()
}

function place(i: number, p: Point) {
  const src = source.value
  if (!src) return
  const next = [...corners.value] as Quad
  next[i] = { x: Math.min(Math.max(p.x, 0), src.width), y: Math.min(Math.max(p.y, 0), src.height) }
  corners.value = next
}

function grab(i: number, e: PointerEvent) {
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  dragging.value = i
  magnify(i)
}

function drag(e: PointerEvent) {
  const i = dragging.value
  const box = stage.value?.getBoundingClientRect()
  if (i < 0 || !box) return
  place(i, {
    x: ((e.clientX - box.left) / box.width) * width.value,
    y: ((e.clientY - box.top) / box.height) * height.value,
  })
  magnify(i)
}

function drop() {
  dragging.value = -1
}

// A finger covers the corner it moves, so the loupe shows what is under it, three times over.
function magnify(i: number) {
  const src = source.value
  const view = loupe.value
  const p = corners.value[i]
  if (!src || !view || !p) return
  const ctx = view.getContext('2d')
  if (!ctx) return
  const zoom = 3
  const span = view.width / zoom
  ctx.clearRect(0, 0, view.width, view.height)
  ctx.drawImage(src, p.x - span / 2, p.y - span / 2, span, span, 0, 0, view.width, view.height)
}

// The arrow keys move a focused corner: a pixel of the photo, or ten with Shift.
function nudge(i: number, e: KeyboardEvent) {
  const step = e.shiftKey ? 10 : 1
  const moves: Record<string, Point> = {
    ArrowLeft: { x: -step, y: 0 },
    ArrowRight: { x: step, y: 0 },
    ArrowUp: { x: 0, y: -step },
    ArrowDown: { x: 0, y: step },
  }
  const move = moves[e.key]
  const p = corners.value[i]
  if (!move || !p) return
  e.preventDefault()
  place(i, { x: p.x + move.x, y: p.y + move.y })
}

async function finish() {
  const src = source.value
  const ctx = src?.getContext('2d')
  if (!src || !ctx) return
  busy.value = true
  // Let the button show it is working before the straightening holds the thread.
  await new Promise((resolve) => setTimeout(resolve, 0))
  const { width, height } = outputSize(corners.value, props.size)
  const straight = warp(ctx.getImageData(0, 0, src.width, src.height), corners.value, width, height)
  const out = document.createElement('canvas')
  out.width = width
  out.height = height
  out.getContext('2d')?.putImageData(new ImageData(straight.data, width, height), 0, 0)
  const blob = await new Promise<Blob | null>((resolve) => out.toBlob(resolve, 'image/jpeg', 0.9))
  busy.value = false
  if (!blob) {
    failed.value = 'The photo could not be prepared. Try again.'
    return
  }
  emit('done', blob)
  visible.value = false
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    :header="`Straighten the ${subject}`"
    :draggable="false"
    :pt="{
      root: {
        class:
          'w-full md:max-w-2xl max-md:h-dvh max-md:max-h-dvh max-md:max-w-none max-md:m-0 max-md:rounded-none',
      },
    }"
  >
    <p class="prose-text mb-3 text-sm text-ink-muted">
      Drag each corner onto a corner of the {{ thing }}. What you mark is straightened into a flat
      rectangle. A focused corner also moves with the arrow keys.
    </p>
    <p v-if="failed" class="mb-3 text-status-danger" role="alert">{{ failed }}</p>
    <p class="mb-3 text-sm" role="status">{{ note }}</p>
    <div class="crop">
      <div ref="stage" class="crop__stage">
        <canvas ref="canvas" class="crop__photo" />
        <svg
          class="crop__overlay"
          :viewBox="`0 0 ${width} ${height}`"
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          <path :d="shade" fill-rule="evenodd" class="crop__shade" />
          <polygon :points="outline" class="crop__edge" />
        </svg>
        <div
          v-show="dragging >= 0"
          class="crop__loupe"
          :class="{ 'crop__loupe--left': (corners[dragging]?.x ?? 0) > width / 2 }"
        >
          <canvas ref="loupe" width="132" height="132" class="block size-full" />
          <span class="crop__cross" />
        </div>
        <button
          v-for="(p, i) in corners"
          :key="i"
          type="button"
          class="crop__corner"
          :style="at(p)"
          :aria-label="names[i]"
          @pointerdown="grab(i, $event)"
          @pointermove="drag"
          @pointerup="drop"
          @pointercancel="drop"
          @keydown="nudge(i, $event)"
        />
      </div>
    </div>
    <template #footer>
      <div class="flex w-full flex-wrap justify-between gap-2">
        <span class="flex gap-2">
          <Button type="button" severity="secondary" @click="rotate"
            ><FontAwesomeIcon :icon="faRotateRight" /> Turn</Button
          >
          <Button type="button" severity="secondary" @click="reset"
            ><FontAwesomeIcon :icon="faArrowsRotate" /> Reset</Button
          >
        </span>
        <span class="flex gap-2">
          <Button type="button" severity="secondary" label="Cancel" @click="visible = false" />
          <Button
            type="button"
            :label="`Use this ${subject}`"
            :loading="busy"
            :disabled="!source"
            @click="finish"
          />
        </span>
      </div>
    </template>
  </Dialog>
</template>

<style scoped>
.crop {
  display: flex;
  justify-content: center;
  touch-action: none;
  user-select: none;
}

.crop__stage {
  position: relative;
  max-width: 100%;
}

/* The photo keeps its shape and fits the dialog: the width there is, and most of the height. */
.crop__photo {
  display: block;
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: 55dvh;
}

@media (width < 48rem) {
  .crop__photo {
    max-height: 62dvh;
  }
}

.crop__overlay {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.crop__shade {
  fill: var(--color-shade);
}

.crop__edge {
  fill: none;
  stroke: var(--color-accent);
  stroke-width: 2;
  vector-effect: non-scaling-stroke;
}

/* A corner is a full target, drawn as a ring around the point it marks. */
.crop__corner {
  position: absolute;
  width: var(--spacing-target);
  height: var(--spacing-target);
  margin: calc(var(--spacing-target) / -2) 0 0 calc(var(--spacing-target) / -2);
  border: 3px solid var(--color-accent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-accent), transparent 80%);
  cursor: grab;
  touch-action: none;
}

.crop__corner:active {
  cursor: grabbing;
}

.crop__loupe {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  width: 8.25rem;
  height: 8.25rem;
  border: 3px solid var(--color-accent);
  border-radius: 999px;
  overflow: hidden;
  background: var(--color-surface-panel);
  pointer-events: none;
}

/* The point under the corner, at the loupe's centre. */
.crop__cross::before,
.crop__cross::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  background: var(--color-accent);
}

.crop__cross::before {
  width: 1.5rem;
  height: 2px;
  transform: translate(-50%, -50%);
}

.crop__cross::after {
  width: 2px;
  height: 1.5rem;
  transform: translate(-50%, -50%);
}

.crop__loupe--left {
  right: auto;
  left: 0.5rem;
}
</style>
