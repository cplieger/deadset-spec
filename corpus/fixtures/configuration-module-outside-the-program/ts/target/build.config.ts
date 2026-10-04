// A second configuration file by name that no compiler configuration includes.
import shared from "./build.shared.js";
import presets from "./presets";

export default [...shared, ...presets];
