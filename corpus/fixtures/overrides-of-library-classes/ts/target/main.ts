class AppError extends Error {
  override name = "AppError";
  constructor(message: string) {
    super(message);
  }
  override toString(): string {
    return `${this.name}: ${this.message}`;
  }
  detail(): string {
    return "detail";
  }
}

throw new AppError("failed");
