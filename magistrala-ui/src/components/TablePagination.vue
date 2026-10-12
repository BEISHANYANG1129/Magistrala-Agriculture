<template>
  <div class="pagination">
    <span v-if="selected">{{ selected }} {{ tr('条已选，共', 'selected of') }} {{ total }}</span>
    <label
      >{{ tr('每页数量', 'Rows per page') }}
      <select :value="size" :disabled="busy" @change="resize(Number($event.target.value))">
        <option v-for="n in pageSizes" :key="n" :value="n">{{ n }}</option>
      </select>
    </label>
    <span>{{ tr(`第 ${page} 页，共 ${pages} 页`, `Page ${page} of ${pages}`) }}</span>
    <button
      type="button"
      class="quiet"
      :disabled="busy || page <= 1"
      :aria-label="tr('首页', 'Go to first page')"
      @click="emit('update:page', 1)"
    >
      «
    </button>
    <button
      type="button"
      class="quiet"
      :disabled="busy || page <= 1"
      :aria-label="tr('上一页', 'Go to previous page')"
      @click="emit('update:page', page - 1)"
    >
      ‹
    </button>
    <button
      type="button"
      class="quiet"
      :disabled="busy || page >= pages"
      :aria-label="tr('下一页', 'Go to next page')"
      @click="emit('update:page', page + 1)"
    >
      ›
    </button>
    <button
      type="button"
      class="quiet"
      :disabled="busy || page >= pages"
      :aria-label="tr('末页', 'Go to last page')"
      @click="emit('update:page', pages)"
    >
      »
    </button>
  </div>
</template>
<script setup>
import { computed, watch } from 'vue'
import { tr } from '../i18n'
const props = defineProps({
  busy: Boolean,
  page: Number,
  size: Number,
  total: Number,
  selected: { type: Number, default: 0 },
})
const emit = defineEmits(['update:page', 'update:size'])
const pageSizes = computed(() => [...new Set([5, 10, 20, 30, 40, 50, props.size])].filter(n => n > 0).sort((a,b) => a-b))
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.size)))
function resize(size) {
  emit('update:size', size)
  emit('update:page', 1)
}
watch(pages, (n) => {
  if (props.page > n) emit('update:page', n)
})
</script>
