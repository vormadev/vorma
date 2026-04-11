/////////////////////////////////////////////////////////////////////
/////// REACT
/////////////////////////////////////////////////////////////////////

// import tailwindcss from "@tailwindcss/vite";
// import react from "@vitejs/plugin-react";
// import { defineConfig } from "vite";
// import vorma from "vorma/vite";

// export default defineConfig({ plugins: [react(), vorma(), tailwindcss()] });

/////////////////////////////////////////////////////////////////////
/////// PREACT
/////////////////////////////////////////////////////////////////////

// import preact from "@preact/preset-vite";
// import tailwindcss from "@tailwindcss/vite";
// import { defineConfig } from "vite";
// import vorma from "vorma/vite";

// export default defineConfig({ plugins: [preact(), vorma(), tailwindcss()] });

/////////////////////////////////////////////////////////////////////
/////// SOLID
/////////////////////////////////////////////////////////////////////

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import vorma from "vorma/vite";

export default defineConfig({ plugins: [solid(), vorma(), tailwindcss()] });
