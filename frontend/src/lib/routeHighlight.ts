import { onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

/**
 * Deep-link highlight helper. List views call useRouteHighlight with a function
 * that returns the container element; after mount (and when the query changes)
 * it scrolls the row tagged `data-hid="<id>"` into view and briefly flashes it.
 *
 * Rows only need a `data-hid="<id>"` attribute for this to work.
 */
export function useRouteHighlight(getContainer?: () => HTMLElement | null | undefined) {
  const route = useRoute()

  function run() {
    const raw = route.query.highlight
    if (!raw) return
    const id = String(raw)
    const container = getContainer ? getContainer() : document.body
    if (!container) return
    const el = container.querySelector<HTMLElement>(`[data-hid="${id}"]`)
    if (!el) return
    el.scrollIntoView({ block: 'center', behavior: 'smooth' })
    el.classList.remove('nb-flash')
    // force reflow so the animation restarts if the same row is highlighted again
    void el.offsetWidth
    el.classList.add('nb-flash')
    window.setTimeout(() => el.classList.remove('nb-flash'), 2400)
  }

  onMounted(() => {
    // wait a tick for async lists to render
    window.setTimeout(run, 60)
  })
  watch(
    () => route.query.highlight,
    () => window.setTimeout(run, 60),
  )
}
