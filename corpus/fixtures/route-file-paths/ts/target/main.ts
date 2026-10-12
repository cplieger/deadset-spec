import { load } from "./src/routes/(app)/+layout.js";
import { GET } from "./dashboard/src/routes/+page.server.js";
import { loader } from "./app/routes/$id.js";
import { handler } from "./pages/api/auth/[...nextauth].js";
import { format } from "./src/lib/date utils/format.js";
import { summary } from "./src/lib/résumé.js";

export const routes = [load, GET, loader, handler, format, summary];
