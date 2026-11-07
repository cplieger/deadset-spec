CHECK := tools/check.ts # tools/unused.ts

build:
	cd web; npx esbuild --bundle app.ts --outdir=../out
	node tools/gen.ts # tools/unused.ts
