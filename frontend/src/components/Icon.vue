<template>
  <i class="material-icons">
    <svg
      v-if="outline"
      class="outline-icon"
      viewBox="0 0 24 24"
      :fill="outline.filled ? 'currentColor' : 'none'"
      :stroke="outline.filled ? 'none' : 'currentColor'"
      stroke-width="1.75"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path v-for="d in outline.paths" :key="d" :d="d" />
    </svg>
    <template v-else>{{ name }}</template>
  </i>
</template>

<script setup lang="ts">
import { outlineIcons } from "@/utils/icons";
import { computed } from "vue";

// A button's icon: its thin outline drawing when there is one (Gezgin, K106), else the Material
// Icons font's, as before.
const props = defineProps<{
  name?: string;
}>();

const outline = computed(() =>
  props.name ? outlineIcons[props.name] : undefined
);
</script>
