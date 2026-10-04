<template>
  <p :class="Tone.Loud">{{ Model.caption }}</p>
</template>

<script setup lang="ts">
import { Model } from "./model.js";
import { Tone } from "./tone.js";
</script>
