// Images the bundler loads as URLs.
declare module "*.svg" {
  const url: string;
  export default url;
}

declare module "*.png" {
  const url: string;
  export default url;
}
