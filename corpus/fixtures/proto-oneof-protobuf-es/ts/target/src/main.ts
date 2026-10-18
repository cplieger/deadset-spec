import { create } from "@bufbuild/protobuf";
import { type Outcome, OutcomeSchema, type Probe, ProbeSchema } from "./gen/v1/outcome_pb.js";

function describe(outcome: Outcome): string {
  if (outcome.result.case === "ok") {
    return outcome.result.value;
  }
  switch (outcome.result.case) {
    case "error":
    case "pending":
      return "unfinished";
    default:
      return "other";
  }
}

function kind(probe: Probe): string | undefined {
  return probe.value.case;
}

console.log(describe(create(OutcomeSchema, { result: { case: "skipped", value: 1 } })), kind(create(ProbeSchema)));
