// run calls the function it is handed, which may count its parameters.
function run(fn) {
  return fn(1, 2);
}

// measured has a length the program reads.
function measured(first, second) {
  return first;
}

// local is called by name only.
function local(first, second) {
  return first;
}

console.log(
  run(function held(first, second) {
    return first;
  }),
  run((first, second = 0, third) => first + second),
  measured.length,
  measured(1, 2),
  local(1, 2),
);
