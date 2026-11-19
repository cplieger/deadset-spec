// A file only a workflow step runs: node reads none of its exports.
class Job {
  run() {
    return 0;
  }
}

export const job = { make: Job };
