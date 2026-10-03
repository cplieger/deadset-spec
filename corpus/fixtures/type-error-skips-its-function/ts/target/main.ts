// helper is called by the function that holds the type errors.
function helper(): number {
  return 1;
}

// Panel is the class the function reaches through a member name that does not
// resolve.
class Panel {
  // count is a member only the unresolved member name could reach.
  count(): number {
    return 2;
  }
}

// broken holds two type errors on one line.
function broken(): number {
  const p = new Panel();
  return helper() + p.cuont() + missing();
}

// dead is called by nothing.
function dead(): number {
  return 3;
}

if (broken() !== 3) {
  throw new Error("broken() is not 3");
}
