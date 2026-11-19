// A configuration file: the tool imports it and reads its default export.
class Plugin {
  static id = "plugin";

  apply() {
    return Plugin.id;
  }
}

// Its methods share the reporter's names, but no export hands a value of it out.
class Lookalike {
  onMutantTested() {
    return 0;
  }

  used() {
    return 1;
  }
}

if (new Lookalike().used() < 0) {
  throw new Error("unreachable");
}

export default { plugins: [new Plugin()] };
