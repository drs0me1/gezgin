<template>
  <div class="rules">
    <div v-for="(rule, index) in rules" :key="index" class="rule-row">
      <select
        class="input"
        :value="rule.allow ? 'allow' : 'block'"
        :aria-label="t('settings.ruleKind')"
        @change="
          update(index, {
            allow: ($event.target as HTMLSelectElement).value === 'allow',
          })
        "
      >
        <option value="block">{{ t("settings.ruleBlock") }}</option>
        <option value="allow">{{ t("settings.ruleAllow") }}</option>
      </select>
      <select
        class="input"
        :value="rule.regex ? 'regex' : 'path'"
        :aria-label="t('settings.ruleMatch')"
        @change="
          update(index, {
            regex: ($event.target as HTMLSelectElement).value === 'regex',
          })
        "
      >
        <option value="path">{{ t("settings.path") }}</option>
        <option value="regex">{{ t("settings.ruleRegex") }}</option>
      </select>
      <input
        v-if="rule.regex"
        class="input"
        type="text"
        :value="rule.regexp.raw"
        :placeholder="t('settings.insertRegex')"
        @keypress.enter.prevent
        @input="
          update(index, {
            regexp: { raw: ($event.target as HTMLInputElement).value },
          })
        "
      />
      <input
        v-else
        class="input"
        type="text"
        :value="rule.path"
        :placeholder="t('settings.insertPath')"
        @keypress.enter.prevent
        @input="
          update(index, { path: ($event.target as HTMLInputElement).value })
        "
      />
      <button
        type="button"
        class="action"
        :aria-label="t('settings.removeRule')"
        :title="t('settings.removeRule')"
        @click="remove(index)"
      >
        <icon name="close" />
      </button>
    </div>
    <button type="button" class="button button--flat rules-add" @click="add">
      <icon name="add" />{{ t("settings.addRule") }}
    </button>
  </div>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
import { useI18n } from "vue-i18n";

// Allow and block rules (Gezgin, K153): what they do, how they match, the path or expression.
const props = defineProps<{
  rules: IRule[];
}>();

const emit = defineEmits<{
  (e: "update:rules", value: IRule[]): void;
}>();

const { t } = useI18n();

const update = (index: number, change: Partial<IRule>) =>
  emit(
    "update:rules",
    props.rules.map((rule, i) => (i === index ? { ...rule, ...change } : rule))
  );

const remove = (index: number) =>
  emit(
    "update:rules",
    props.rules.filter((_, i) => i !== index)
  );

const add = () =>
  emit("update:rules", [
    ...props.rules,
    { allow: false, path: "", regex: false, regexp: { raw: "" } },
  ]);
</script>
