export const page = <p>
  // deadset:ignore DS1002 -- JSX text, not a comment
</p>; function unused(): void {}
export const note = <b>// deadset:ignore malformed</b>;

console.log(page, note);
