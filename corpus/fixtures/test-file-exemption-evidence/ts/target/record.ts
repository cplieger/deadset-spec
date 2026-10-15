// Sample is written by its initializer and serialized by the test file alone.
export class Sample {
  id = 1;
  name = "sample";
}

// Draft is serialized by nothing.
export class Draft {
  id = 2;
  label?: string;
}
