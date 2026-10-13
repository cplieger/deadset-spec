declare const names: string[];

for (const name of names) {
  await import(`./migrations/${name}`);
  await import(`./${name}`);
}
