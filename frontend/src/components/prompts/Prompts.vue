<template>
  <base-modal v-if="modal != null" :prompt="currentPromptName" @closed="close">
    <keep-alive>
      <component :is="modal" />
    </keep-alive>
  </base-modal>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";
import { useLayoutStore } from "@/stores/layout";

import BaseModal from "./BaseModal.vue";
import Help from "./Help.vue";
import Info from "./Info.vue";
import Delete from "./Delete.vue";
import Download from "./Download.vue";
import Rename from "./Rename.vue";
import Move from "./Move.vue";
import Copy from "./Copy.vue";
import NewFile from "./NewFile.vue";
import NewDir from "./NewDir.vue";
import Replace from "./Replace.vue";
import Share from "./Share.vue";
import ShareDelete from "./ShareDelete.vue";
import ShareEdit from "./ShareEdit.vue";
import Confirm from "./Confirm.vue";
import ShareInfo from "./ShareInfo.vue";
import Upload from "./Upload.vue";
import DiscardEditorChanges from "./DiscardEditorChanges.vue";
import EditorConflict from "./EditorConflict.vue";
import ResolveConflict from "./ResolveConflict.vue";
import CurrentPassword from "./CurrentPassword.vue";
import Extract from "./Extract.vue";
import Archive from "./Archive.vue";

const layoutStore = useLayoutStore();

const { currentPromptName } = storeToRefs(layoutStore);

const components = new Map<string, any>([
  ["info", Info],
  ["help", Help],
  ["delete", Delete],
  ["rename", Rename],
  ["move", Move],
  ["copy", Copy],
  ["newFile", NewFile],
  ["newDir", NewDir],
  ["download", Download],
  ["replace", Replace],
  ["share", Share],
  ["upload", Upload],
  ["share-delete", ShareDelete],
  ["share-edit", ShareEdit],
  ["confirm", Confirm],
  ["share-info", ShareInfo],
  ["discardEditorChanges", DiscardEditorChanges],
  ["editorConflict", EditorConflict],
  ["resolve-conflict", ResolveConflict],
  ["current-password", CurrentPassword],
  ["extract", Extract],
  ["archive", Archive],
]);

const modal = computed(() => {
  const modal = components.get(currentPromptName.value!);
  if (!modal) null;

  return modal;
});

const close = () => {
  if (!layoutStore.currentPrompt) return;
  layoutStore.closeHovers();
};
</script>
