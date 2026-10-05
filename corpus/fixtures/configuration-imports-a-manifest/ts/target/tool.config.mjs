import manifest from "./package.json" with { type: "json" };

export default { name: manifest.name };
