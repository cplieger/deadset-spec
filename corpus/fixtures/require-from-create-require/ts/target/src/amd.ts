const widget = String(Date.now());
define(["require"], (require) => {
  console.log(require("./widgets/" + widget));
});
