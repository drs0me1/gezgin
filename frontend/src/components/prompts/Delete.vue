<template>
  <div class="card floating">
    <div class="card-content">
      <p v-if="!this.isListing || selectedCount === 1">
        {{ $t("prompts.deleteMessageSingle") }}
      </p>
      <p v-else>
        {{ $t("prompts.deleteMessageMultiple", { count: selectedCount }) }}
      </p>
      <p class="small">{{ $t("trash.deleteHint") }}</p>
    </div>
    <div class="card-action">
      <button
        @click="closeHovers"
        class="button button--flat button--grey"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
        tabindex="2"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        @click="submit(true)"
        class="button button--flat button--red"
        :aria-label="$t('trash.deletePermanently')"
        :title="$t('trash.deletePermanently')"
        tabindex="3"
      >
        {{ $t("trash.deletePermanently") }}
      </button>
      <button
        id="focus-prompt"
        @click="submit(false)"
        class="button button--flat"
        :aria-label="$t('trash.moveToTrash')"
        :title="$t('trash.moveToTrash')"
        tabindex="1"
      >
        {{ $t("trash.moveToTrash") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mapActions, mapState, mapWritableState } from "pinia";
import { files as api } from "@/api";
import { StatusError } from "@/api/utils";
import buttons from "@/utils/buttons";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

export default {
  name: "delete",
  inject: ["$showError"],
  computed: {
    ...mapState(useFileStore, [
      "isListing",
      "selectedCount",
      "req",
      "selected",
    ]),
    ...mapState(useLayoutStore, ["currentPrompt"]),
    ...mapWritableState(useFileStore, ["reload", "preselect"]),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    submit: async function (permanent) {
      buttons.loading("delete");

      try {
        if (!this.isListing) {
          await api.remove(this.$route.path, permanent);
          buttons.success("delete");

          this.currentPrompt?.confirm();
          this.closeHovers();
          return;
        }

        this.closeHovers();

        if (this.selectedCount === 0) {
          return;
        }

        const promises = [];
        for (const index of this.selected) {
          promises.push(api.remove(this.req.items[index].url, permanent));
        }

        await Promise.all(promises);
        buttons.success("delete");

        const nearbyItem =
          this.req.items[Math.max(0, Math.min(this.selected) - 1)];

        this.preselect = nearbyItem?.path;

        this.reload = true;
      } catch (e) {
        buttons.done("delete");
        // 409: on another disk than the trash; only a permanent delete can remove it.
        this.$showError(
          !permanent && e instanceof StatusError && e.status === 409
            ? this.$t("trash.otherDisk")
            : e
        );
        if (this.isListing) this.reload = true;
      }
    },
  },
};
</script>
