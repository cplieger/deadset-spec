import {
  ByAlias,
  ByBag,
  ByCondA,
  ByCondB,
  ByDestructure,
  ByDirectCond,
  ByAnonymous,
  ByArray,
  ByArrayBranch,
  ByArrayCopy,
  ByArrow,
  ByExpression,
  ByField,
  ByFunction,
  ByMethod,
  ByLater,
  ByNullish,
  ByObjectBranch,
  ByParam,
  ByPropCopy,
  ByProperty,
  ByRest,
  ByReturned,
  ByThis,
  ByTwoWrappers,
  Described,
} from "./elements.js";
import {
  describe,
  Hosted,
  outer,
  register,
  register as reg,
  registerArrow,
  registerExpression,
  registerLoose,
  registerRest,
  Registry,
  table,
} from "./register.js";

register("x-function", ByFunction);
registerArrow("x-arrow", ByArrow);
new Registry().add("x-method", ByMethod);
outer(ByTwoWrappers);
registerRest("x-rest", ByRest);
table.add("x-property", ByProperty);
registerExpression(ByExpression);
reg("x-alias", ByAlias);
const held: [typeof ByArray] = [ByArray];
outer(...held);
new Hosted().add(ByThis);

declare const flag: boolean;
declare const maybe: CustomElementConstructor | undefined;
register("x-cond", flag ? ByCondA : ByCondB);
register("x-nullish", maybe ?? ByNullish);
const regs = { reg: register };
regs.reg("x-prop-copy", ByPropCopy);
customElements.define("x-direct", flag ? ByDirectCond : ByDirectCond);
class Box {
  reg = register;
}
new Box().reg("x-field", ByField);
const bag = { register };
bag.register("x-bag", ByBag);
const wrapped = { register };
const { register: renamed } = wrapped;
renamed("x-destructure", ByDestructure);

function use(r: typeof register): void {
  r("x-param", ByParam);
}
use(register);
const [copied] = [register];
copied("x-array-copy", ByArrayCopy);
function getRegister(): typeof register {
  return register;
}
getRegister()("x-returned", ByReturned);
let later: typeof register | undefined;
later = register;
later("x-later", ByLater);
const anonymous = [
  (name: string, element: CustomElementConstructor): void => customElements.define(name, element),
];
anonymous[0]?.("x-anonymous", ByAnonymous);
registerLoose("x-array-branch", [flag ? ByArrayBranch : ByArrayBranch]);
registerLoose("x-object-branch", { element: flag ? ByObjectBranch : ByObjectBranch });

export const named = describe(Described);
