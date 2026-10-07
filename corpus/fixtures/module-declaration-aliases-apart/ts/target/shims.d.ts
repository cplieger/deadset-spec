// Two virtual modules, each exporting its default as a separate statement.
declare module "virtual:theme" {
  const theme: { dark: boolean };
  export const version: typeof theme;
  export default theme;
}

declare module "virtual:icons" {
  const icons: string[];
  export const count: typeof icons;
  export default icons;
}
