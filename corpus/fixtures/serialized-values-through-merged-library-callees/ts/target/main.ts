import "./globals.js";

// Each call resolves to the library's signature, whose code runs.
class Mark {
  id = 1;
}
console.log(new Mark());

class Sent {
  id = 1;
}
function send(value: unknown): string {
  return JSON.stringify(value);
}
console.log(send.call(undefined, new Sent()));
