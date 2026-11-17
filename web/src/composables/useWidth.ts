import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

/**
 * How wide something on the page is, in CSS pixels, kept up as the page changes: what `measure`
 * reads from the element watched, once it is drawn and whenever it changes size; 0 before.
 * A drawing to scale, or one that fills the page's width, sizes itself by it.
 */
export function useWidth(
  watched: Readonly<Ref<HTMLElement | null>>,
  measure: (el: HTMLElement) => number = (el) => el.clientWidth,
): Ref<number> {
  const width = ref(0)
  const read = () => {
    if (watched.value) {
      width.value = measure(watched.value)
    }
  }
  let observer: ResizeObserver | undefined
  onMounted(() => {
    read()
    observer = new ResizeObserver(read)
    if (watched.value) {
      observer.observe(watched.value)
    }
  })
  onBeforeUnmount(() => observer?.disconnect())
  return width
}
