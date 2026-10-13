// component receives the class it decorates.
function component(_value: unknown, _context: ClassDecoratorContext): void {}

@component
class View {
  constructor(
    private readonly dialog: string,
    private readonly title: string,
  ) {}
  shown(): string {
    return this.title;
  }
}

export const shown = new View("d", "t").shown();
