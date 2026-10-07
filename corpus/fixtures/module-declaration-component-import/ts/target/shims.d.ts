// The type every component file's default export has.
declare module "*.vue" {
  const component: { name: string };
  export const meta: string;
  export default component;
}
