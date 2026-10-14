interface ToolOptions {}

type Strict = "strict" extends keyof ToolOptions ? true : false;

interface ImportMeta {
  readonly strict: Strict;
}
