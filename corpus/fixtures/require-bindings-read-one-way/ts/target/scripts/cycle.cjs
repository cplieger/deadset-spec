var a = b;
var b = a;

a("esbuild").build({ entryPoints: ["src/held.ts"] });
