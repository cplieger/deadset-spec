<template><p>{{ a }}</p></template>
<script lang="ts">
class Row {
  used = 1;
  dead = 2;
}
export const a = new Row().used;
</script>
