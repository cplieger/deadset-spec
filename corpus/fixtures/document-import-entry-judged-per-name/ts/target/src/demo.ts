export type DemoProps = {
  readonly label: string;
};

export function Demo(props: DemoProps): string {
  return props.label;
}
