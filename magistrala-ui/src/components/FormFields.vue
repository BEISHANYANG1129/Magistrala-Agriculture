<template>
  <div v-if="schema.type === 'object' && schema.properties" class="form-grid">
    <template v-for="(field, key) in schema.properties" :key="key">
      <label
        v-if="!hidden.includes(key)"
        class="field"
        :class="{ wide: ['object', 'array', 'json'].includes(field.type) }"
      >
        <span>{{ label(key) }} <b v-if="isRequired(key, field)">*</b></span>
        <FormFields
          :schema="{
            ...field,
            required: Array.isArray(field.required) ? field.required : isRequired(key, field),
          }"
          :name="key"
          :model-value="modelValue?.[key]"
          :choices="choices"
          @update:model-value="(value) => change(key, value)"
        />
      </label>
    </template>
  </div>
  <select
    v-else-if="schema.enum"
    :value="modelValue ?? ''"
    :required="schema.required === true"
    @change="emit('update:modelValue', $event.target.value || undefined)"
  >
    <option value="">{{ tr('请选择', 'Select') }}</option>
    <option v-for="v in schema.enum" :key="v" :value="v">{{ label(v) }}</option>
  </select>
  <select
    v-else-if="schema.type === 'boolean'"
    :value="modelValue === undefined ? '' : String(modelValue)"
    :required="schema.required === true"
    @change="
      emit(
        'update:modelValue',
        $event.target.value === '' ? undefined : $event.target.value === 'true',
      )
    "
  >
    <option value="">{{ tr('默认', 'Default') }}</option>
    <option value="true">{{ tr('是', 'Yes') }}</option>
    <option value="false">{{ tr('否', 'No') }}</option>
  </select>
  <div v-else-if="schema.type === 'array'" class="array-field">
    <div v-for="(value, index) in modelValue || []" :key="index" class="array-row">
      <FormFields
        :schema="schema.items || { type: 'json' }"
        :name="name"
        :model-value="value"
        :choices="choices"
        @update:model-value="(v) => changeItem(index, v)"
      />
      <button
        type="button"
        class="quiet"
        @click="
          emit(
            'update:modelValue',
            modelValue.filter((_, i) => i !== index),
          )
        "
      >
        {{ tr('移除', 'Remove') }}
      </button>
    </div>
    <button
      type="button"
      class="quiet"
      @click="
        emit('update:modelValue', [
          ...(modelValue || []),
          schema.items?.type === 'object' ? {} : '',
        ])
      "
    >
      + {{ tr('添加一项', 'Add item') }}
    </button>
  </div>
  <textarea
    v-else-if="schema.type === 'json' || schema.type === 'object'"
    :value="jsonText"
    rows="5"
    spellcheck="false"
    @input="parseJSON($event)"
    :required="schema.required === true"
    placeholder="{}"
  ></textarea>
  <input
    v-else-if="schema.format === 'binary'"
    type="file"
    @change="emit('update:modelValue', $event.target.files[0])"
    :required="schema.required === true"
  />
  <textarea
    v-else-if="/script|template|content|certificate|csr|logic|description/i.test(name)"
    :value="modelValue ?? ''"
    rows="4"
    @input="emit('update:modelValue', $event.target.value || undefined)"
    :required="schema.required === true"
  ></textarea>
  <span v-else class="input-wrap">
    <input
      :value="modelValue ?? ''"
      :type="
        schema.type === 'number' || schema.type === 'integer'
          ? 'number'
          : /password|secret|private.*key/i.test(name)
            ? 'password'
            : 'text'
      "
      :step="schema.type === 'integer' ? '1' : 'any'"
      :min="schema.minimum"
      :max="schema.maximum"
      :list="options.length ? listId : undefined"
      :required="schema.required === true"
      :autocomplete="/password|secret/i.test(name) ? 'new-password' : 'off'"
      @input="
        emit(
          'update:modelValue',
          $event.target.value === ''
            ? undefined
            : ['number', 'integer'].includes(schema.type)
              ? Number($event.target.value)
              : $event.target.value,
        )
      "
    />
    <datalist v-if="options.length" :id="listId">
      <option v-for="v in options" :key="v.id" :value="v.id">
        {{ v.name || v.displayName || v.id }}
      </option>
    </datalist>
  </span>
</template>
<script setup>
import { computed, useId } from 'vue'
import { label, tr } from '../i18n'
const props = defineProps({
  schema: { type: Object, required: true },
  modelValue: {},
  name: { type: String, default: '' },
  hidden: { type: Array, default: () => [] },
  choices: { type: Object, default: () => ({}) },
})
const emit = defineEmits(['update:modelValue'])
const listId = useId()
const options = computed(() => props.choices[props.name] || [])
const jsonText = computed(() =>
  props.modelValue === undefined ? '' : JSON.stringify(props.modelValue, null, 2),
)
function isRequired(key, field) {
  return Array.isArray(props.schema.required)
    ? props.schema.required.includes(key)
    : field.required === true
}
function change(key, value) {
  emit('update:modelValue', { ...(props.modelValue || {}), [key]: value })
}
function changeItem(index, value) {
  const list = [...(props.modelValue || [])]
  list[index] = value
  emit('update:modelValue', list)
}
function parseJSON(event) {
  try {
    const v = event.target.value.trim()
    const value = v ? JSON.parse(v) : undefined
    event.target.setCustomValidity('')
    emit('update:modelValue', value)
  } catch {
    event.target.setCustomValidity(tr('请输入有效的 JSON。', 'Enter valid JSON.'))
  }
}
</script>
