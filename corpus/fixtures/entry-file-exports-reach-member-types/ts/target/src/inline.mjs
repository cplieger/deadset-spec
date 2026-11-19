// A file an inline module script of an HTML file imports: an entry file, as one ts.entry_files names.
class Panel {
  draw() {
    return 0;
  }
}

export const panels = [{ widget: Panel }];

export function helper(list) {
  return list.length + local();
}

export function local() {
  return 1;
}

helper(panels);
