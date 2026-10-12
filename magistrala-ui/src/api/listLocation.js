import { nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

// Keep list state in the URL so detail links and browser history restore the same view.
export function useListLocation({ page, size, filters, active = () => true }) {
  const route = useRoute(), router = useRouter(), restoring = ref(false)
  let revision = 0
  watch(() => [route.path, route.query], async () => {
    if (!active()) return
    const current = ++revision
    restoring.value = true
    for (const [key, [value, fallback]] of Object.entries(filters)) value.value = String(route.query[key] ?? fallback)
    const limit = Number(route.query.limit || 10), offset = Number(route.query.offset || 0)
    size.value = [5,10,20,30,40,50].includes(limit) ? limit : 10
    page.value = Number.isFinite(offset) && offset >= 0 ? Math.floor(offset / size.value) + 1 : 1
    await nextTick()
    if (revision === current) restoring.value = false
  }, { immediate: true })
  watch(Object.values(filters).map(([value]) => value), () => {
    if (!restoring.value) page.value = 1
  }, { flush: 'sync' })
  watch([page, size, ...Object.values(filters).map(([value]) => value)], () => {
    if (restoring.value || !active()) return
    const query = { ...route.query, limit: String(size.value), offset: String((page.value - 1) * size.value) }
    for (const [key, [value]] of Object.entries(filters)) query[key] = value.value || undefined
    const same = Object.keys(query).every(key => String(query[key] ?? '') === String(route.query[key] ?? ''))
    if (!same) router.replace({ query })
  }, { flush: 'post' })
  return restoring
}
