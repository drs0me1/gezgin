<template>
  <div class="card floating" id="share-info">
    <div class="card-title">
      <h2>{{ t("shares.info") }}</h2>
    </div>
    <div class="card-content">
      <p>
        <strong>{{ t("shares.name") }}:</strong> {{ props.name }}
      </p>
      <p v-if="props.folder">
        <strong>{{ t("favorites.location") }}:</strong> {{ props.folder }}
      </p>
      <p>
        <strong>{{ t("shares.kind") }}:</strong> {{ props.kind }}
        <template v-if="share?.kind === 'webdav'">
          · {{ t("shares.webdavUser", { user: share.webdavUser }) }}
        </template>
      </p>
      <p class="address">
        <strong>{{ t("shares.address") }}:</strong>
        <a
          v-if="share?.kind !== 'webdav'"
          :href="props.url"
          target="_blank"
          rel="noopener noreferrer"
          >{{ props.url }}</a
        >
        <span v-else>{{ props.url }}</span>
      </p>
      <p>
        <strong>{{ t("shares.end") }}:</strong> {{ props.end }}
        <template v-if="props.endDate"> ({{ props.endDate }})</template>
      </p>
      <p>
        <strong>{{ t("shares.password") }}:</strong>
        {{
          share?.hasPassword ? t("shares.passwordYes") : t("shares.passwordNo")
        }}
      </p>
      <p v-if="props.owner">
        <strong>{{ t("shares.owner") }}:</strong> {{ props.owner }}
      </p>
    </div>
    <div class="card-action">
      <button
        id="focus-prompt"
        class="button button--flat"
        @click="layoutStore.closeHovers"
        :aria-label="t('buttons.ok')"
        :title="t('buttons.ok')"
      >
        {{ t("buttons.ok") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { SharedItem } from "@/api/share";
import { useLayoutStore } from "@/stores/layout";
import { useI18n } from "vue-i18n";

// A share's information (Gezgin, K133): its item and place, how it is shared and at which
// address, until when, with a password or not, and by whom.
const { t } = useI18n();
const layoutStore = useLayoutStore();
const props: {
  share?: SharedItem;
  name?: string;
  folder?: string;
  kind?: string;
  url?: string;
  end?: string;
  endDate?: string;
  owner?: string;
} = layoutStore.currentPrompt?.props ?? {};
const share = props.share;
</script>

<style scoped>
#share-info .card-content p {
  margin: 0.4em 0;
  word-break: break-word;
}

#share-info .address a {
  color: var(--blue);
}
</style>
