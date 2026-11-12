<template>
  <div class="message-image">
    <button v-if="!failed" class="image-preview quiet" type="button" :aria-label="tr('放大图片', 'Enlarge image')" @click="expanded = true">
      <img :src="image.src" :alt="label" loading="lazy" @error="failed = true" />
    </button>
    <p v-else class="error">{{ tr('图片无法解码，请检查消息内容。', 'Image cannot be decoded. Check the message content.') }}</p>
    <div class="actions">
      <small>{{ (image.bytes / 1024).toFixed(1) }} KB</small>
      <a :href="image.src" :download="filename">{{ tr('下载图片', 'Download image') }}</a>
    </div>
    <ModalDialog v-if="expanded" @close="expanded = false">
      <h2>{{ label }}</h2>
      <img class="image-full" :src="image.src" :alt="label" />
      <a :href="image.src" :download="filename">{{ tr('下载图片', 'Download image') }}</a>
    </ModalDialog>
  </div>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import ModalDialog from './ModalDialog.vue'
import { tr } from '../i18n'
const props = defineProps({ image: { type: Object, required: true }, label: { type: String, default: '' } })
const expanded = ref(false), failed = ref(false)
const filename = computed(() => `${(props.label || 'snapshot').replace(/[^\p{L}\p{N}._-]/gu, '_').slice(0, 100)}.${props.image.extension}`)
watch(() => props.image.src, () => { failed.value = false; expanded.value = false })
</script>
<style scoped>
.message-image { min-width: 180px; }
.image-preview { display: block; padding: 0; border: 0; background: #101820; overflow: hidden; border-radius: 8px; }
.image-preview img { display: block; width: 240px; max-width: 100%; max-height: 240px; object-fit: contain; }
.image-full { display: block; max-width: 100%; max-height: 70vh; margin: 12px auto; object-fit: contain; }
.actions { margin-top: 8px; gap: 16px; }
</style>
