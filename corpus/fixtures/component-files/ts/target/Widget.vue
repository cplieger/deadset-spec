<template>
  <p>{{ format(model.title()) }}</p>
</template>

<script setup lang="ts">
import { format } from "./format.js";
import { Model } from "./model.js";

const model = new Model();

function localHelper(): string {
  return "named by no block";
}
</script>
