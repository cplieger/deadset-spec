// Named only by a call through a parameter that shadows the bundler's binding.

export function shadowed(): string {
  return "never built";
}
