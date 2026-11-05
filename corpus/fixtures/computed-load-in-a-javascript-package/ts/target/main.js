const names = process.argv.slice(2);

for (const name of names) {
  await import(`./plugins/${name}.js`);
}
