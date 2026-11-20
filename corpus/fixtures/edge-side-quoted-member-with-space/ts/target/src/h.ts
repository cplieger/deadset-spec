// Headers names its method by a string that is not an identifier name.
export class Headers {
  // The provides side of the edge headers/content-type.
  'content type'(): string {
    return "text/plain";
  }
}
