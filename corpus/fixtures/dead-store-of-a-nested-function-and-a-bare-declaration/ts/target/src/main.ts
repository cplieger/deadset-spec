function outer(n: number): number {
  function unused(): number {
    return 1;
  }
  function recursive(k: number): number {
    return k > 0 ? recursive(k - 1) : 0;
  }
  var never;
  return n + later();
  function later(): number {
    return 2;
  }
}

console.log(outer(1));
