// The class the lifecycle contract names as the base of its components.
class Base {}

// A component by its base, whose lifecycle member the framework calls.
export class Widget extends Base {
  attach(): void {}
}
