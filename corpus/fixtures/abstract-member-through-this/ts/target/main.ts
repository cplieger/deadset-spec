// Processor calls a member its subclasses provide.
abstract class Processor {
  run(): string {
    return this.cookies();
  }

  abstract cookies(): string;
}

// LambdaProcessor provides the member Processor calls.
class LambdaProcessor extends Processor {
  cookies(): string {
    return "c";
  }

  spare(): string {
    return "s";
  }
}

export const out = new LambdaProcessor().run();
