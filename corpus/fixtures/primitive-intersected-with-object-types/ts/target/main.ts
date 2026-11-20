import { model } from "./model.js";

for (const fact of model.facts.values()) {
  console.log(fact.alias, fact.counts, fact.flag, fact.tag);
}
