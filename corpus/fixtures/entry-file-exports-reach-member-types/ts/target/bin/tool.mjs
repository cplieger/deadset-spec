#!/usr/bin/env node
// A file only the manifest's bin names: node runs it as a command and reads none of its exports.
class Tool {
  go() {
    return 0;
  }
}

export const tool = { make: Tool };
