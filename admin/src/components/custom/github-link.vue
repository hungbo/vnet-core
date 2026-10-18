<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import WebSiteLink from './web-site-link.vue';

defineOptions({ name: 'GithubLink' });

interface Props {
  /** github link */
  link: string;
}

const props = defineProps<Props>();

const { t } = useI18n();
</script>

<template>
  <WebSiteLink :label="t('about.projectInfo.githubLink') + ':'" :link="link" />
</template>

<style scoped></style>
