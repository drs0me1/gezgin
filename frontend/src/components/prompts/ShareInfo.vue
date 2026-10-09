<template>
  <div class="card floating" id="share-info">
    <div class="card-title">
      <h2>{{ t("shares.info") }}</h2>
    </div>
    <div class="card-content">
      <!-- What is handed to others first, each to be copied (Gezgin, K136). -->
      <label class="field-label" for="share-info-url">
        {{ isWebDAV ? t("shares.address") : t("shares.link") }}
      </label>
      <div class="copy-field">
        <input
          id="share-info-url"
          class="input"
          type="text"
          readonly
          :value="props.url"
          @focus="selectAll"
        />
        <button
          class="action"
          @click="copyText(props.url ?? '', t('success.linkCopied'))"
          :aria-label="
            isWebDAV ? t('shares.copyAddress') : t('shares.copyLink')
          "
          :title="isWebDAV ? t('shares.copyAddress') : t('shares.copyLink')"
        >
          <icon name="content_copy" />
        </button>
      </div>
      <template v-if="isWebDAV">
        <label class="field-label" for="share-info-user">
          {{ t("prompts.webdavUser") }}
        </label>
        <div class="copy-field">
          <input
            id="share-info-user"
            class="input"
            type="text"
            readonly
            :value="share?.webdavUser"
            @focus="selectAll"
          />
          <button
            class="action"
            @click="copyText(share?.webdavUser ?? '', t('shares.userCopied'))"
            :aria-label="t('shares.copyUser')"
            :title="t('shares.copyUser')"
          >
            <icon name="content_copy" />
          </button>
        </div>
      </template>

      <dl class="facts">
        <dt>{{ t("shares.name") }}</dt>
        <dd>{{ props.name }}</dd>
        <template v-if="props.folder">
          <dt>{{ t("favorites.location") }}</dt>
          <dd>{{ props.folder }}</dd>
        </template>
        <dt>{{ t("shares.kind") }}</dt>
        <dd>{{ props.kind }}</dd>
        <dt>{{ t("shares.end") }}</dt>
        <dd>
          {{ props.end }}
          <template v-if="props.endDate"> ({{ props.endDate }})</template>
        </dd>
        <dt>{{ t("shares.password") }}</dt>
        <dd>
          {{
            share?.hasPassword
              ? t("shares.passwordYes")
              : t("shares.passwordNo")
          }}
        </dd>
        <template v-if="props.owner">
          <dt>{{ t("shares.owner") }}</dt>
          <dd>{{ props.owner }}</dd>
        </template>
      </dl>
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
import Icon from "@/components/Icon.vue";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { inject } from "vue";
import { useI18n } from "vue-i18n";

// A share's information (Gezgin, K133, K136): at the top what is handed to others, the address
// and a WebDAV share's username, each in a field of its own to be copied; under them its item and
// place, kind, end, password or none, and owner. Long values stay in their field, so that the
// window fits a phone.
const { t } = useI18n();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
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
const isWebDAV = share?.kind === "webdav";

const selectAll = (event: FocusEvent) =>
  (event.target as HTMLInputElement).select();

const copyText = (text: string, done: string) => {
  copy({ text }).then(
    () => $showSuccess(done),
    () =>
      copy({ text }, { permission: true }).then(
        () => $showSuccess(done),
        (e) => $showError(e)
      )
  );
};
</script>

<style scoped>
#share-info .field-label {
  display: block;
  margin: 0 0 0.3em;
  font-size: 0.85em;
  color: var(--textPrimary);
}

#share-info .copy-field {
  display: flex;
  align-items: center;
  gap: 0.3em;
  margin: 0 0 0.9em;
}

#share-info .copy-field .input {
  flex: 1 1 auto;
  min-width: 0;
  margin: 0;
  font-family: monospace;
  font-size: 0.9em;
  text-overflow: ellipsis;
}

#share-info .copy-field .action {
  flex: 0 0 auto;
}

#share-info .facts {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 0.35em 0.8em;
  margin: 0.4em 0 0;
  padding-top: 0.8em;
  border-top: 1px solid var(--borderSecondary);
}

#share-info .facts dt {
  color: var(--textPrimary);
}

#share-info .facts dd {
  margin: 0;
  color: var(--textSecondary);
  overflow-wrap: anywhere;
}
</style>
