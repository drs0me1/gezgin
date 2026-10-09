<template>
  <div
    class="context-menu"
    ref="contextMenu"
    v-show="show"
    :style="{
      top: `${top}px`,
      left: `${left}px`,
    }"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, watch, onUnmounted } from "vue";

const emit = defineEmits(["hide"]);
const props = defineProps<{ show: boolean; pos: { x: number; y: number } }>();
const contextMenu = ref<HTMLElement | null>(null);

// The menu stays in the window (Gezgin): measured once shown, it moves left of a pointer near the
// right edge and up from one near the bottom. The position is the page's (pos.y counts the
// scroll).
const left = ref<number>(0);
const top = ref<number>(0);
const margin = 8;
const place = async () => {
  left.value = props.pos.x;
  top.value = props.pos.y;
  await nextTick();
  const menu = contextMenu.value;
  if (!menu) return;
  const width = menu.offsetWidth;
  const height = menu.offsetHeight;
  left.value = Math.max(
    margin,
    Math.min(props.pos.x, window.innerWidth - width - margin)
  );
  // Up only as far as it must, and never under the fixed header.
  const header = document.querySelector("header")?.offsetHeight ?? 0;
  const highest = window.scrollY + header + margin;
  const bottom = window.scrollY + window.innerHeight - margin;
  top.value = Math.max(highest, Math.min(props.pos.y, bottom - height));
};
watch(
  () => [props.show, props.pos.x, props.pos.y],
  () => {
    if (props.show) place();
  }
);

const hideContextMenu = () => {
  emit("hide");
};

watch(
  () => props.show,
  (val) => {
    if (val) {
      document.addEventListener("click", hideContextMenu);
    } else {
      document.removeEventListener("click", hideContextMenu);
    }
  }
);

onUnmounted(() => {
  document.removeEventListener("click", hideContextMenu);
});
</script>
