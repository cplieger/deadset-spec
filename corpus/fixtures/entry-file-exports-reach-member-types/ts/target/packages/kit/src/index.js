// A published workspace member: a consumer holds the object its entry exports.
class Kit {
  static made = 1;

  run() {
    return 1;
  }
}

export const kit = { factory: Kit };
