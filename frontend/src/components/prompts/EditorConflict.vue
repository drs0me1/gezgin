<template>
  <div class="card floating">
    <div class="card-content">
      <p>{{ $t("editor.conflict") }}</p>
    </div>
    <div class="card-action">
      <button
        id="focus-prompt"
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
        tabindex="1"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        class="button button--flat button--red"
        @click="currentPrompt.confirm"
        :aria-label="$t('editor.overwrite')"
        :title="$t('editor.overwrite')"
        tabindex="2"
      >
        {{ $t("editor.overwrite") }}
      </button>
    </div>
  </div>
</template>

<script>
import { useLayoutStore } from "@/stores/layout";
import { mapActions, mapState } from "pinia";

export default {
  name: "editorConflict",
  computed: {
    ...mapState(useLayoutStore, ["currentPrompt"]),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
  },
};
</script>
