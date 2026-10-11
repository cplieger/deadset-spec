function sealed(value: unknown, context: ClassDecoratorContext): void {
  void value;
  void context;
}

// deadset:ignore DS1002 -- The generator reads this value from source.
// pin: datasource=git-refs name=example-list
const listCommit = "v2";

// deadset:ignore DS1002 -- Registered by name in the plugin table.
@sealed
class AboveDecorator {}

@sealed
// deadset:ignore DS1002 -- Registered by name in the plugin table.
class BetweenDecoratorAndClass {}

@sealed // deadset:ignore DS1002 -- Registered by name in the plugin table.
class TrailingDecorator {}
