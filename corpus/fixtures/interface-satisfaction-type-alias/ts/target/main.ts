type Shape = { readonly kind: string; area(): number };

// Square reaches Shape through its implements clause and the array of shapes.
class Square implements Shape {
  readonly kind = "square";
  area(): number {
    return 1;
  }
  spare(): number {
    return 0;
  }
}

const shapes: Shape[] = [new Square()];
export const kinds = shapes.map((one) => one.kind + String(one.area()));
