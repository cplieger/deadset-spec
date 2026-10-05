// boot is called by the worker's own top level, which the runtime runs.
function boot(): void {}

boot();

// handle is an export of the worker that nothing imports.
export function handle(): void {}
