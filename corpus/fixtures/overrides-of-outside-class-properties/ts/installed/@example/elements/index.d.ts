/** An element a host page renders. */
export declare class Element {
  static styles?: string;
  static properties: Record<string, unknown>;
  protected get renderRoot(): object;
  label: string;
  update(): void;
}
