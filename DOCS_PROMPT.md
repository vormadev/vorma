## Packages

Review the provided package code for production-readiness. If production-ready,
generate a minimal markdown API reference document with the following
requirements:

1. Frontmatter: Include title (package name) and description (one-line summary).
2. Import: Single-line import statement at the top, using the full module path,
   e.g., github.com/vormadev/vorma/<path>/<to>/<package>.
3. API coverage: All exported functions/types/constants/what-have-you with full
   signatures.
4. Descriptions: Brief usage descriptions placed before their associated code
   blocks.
5. Examples: Include minimum viable usage examples only when usage is
   non-obvious.
6. Optimization target: LLM consumption. Prioritize brevity and clarity over
   human-friendly prose.

If not production-ready, report issues instead of generating documentation.

## Whole Framework

Generate a minimal markdown API reference document for the Vorma framework. I
have included all source code, as well as the bootstrapper, as well as some
internals, just so you understand them and have full context. However, I want
this to just cover the actual Vorma API surface.

1. Frontmatter: Include title and one-line description.
2. API coverage: All exported functions/types/constants/what-have-you with full
   signatures.
3. Descriptions: Brief usage descriptions placed before their associated code
   blocks.
4. Examples: Include minimum viable usage examples only when usage is
   non-obvious.
5. Optimization target: LLM consumption. Prioritize brevity and clarity over
   human-friendly prose.

The end result should be enough for an LLM to understand the framework, it's
default project structure, and its full API surface, on both the server and the
client (and using any client UI library supported), but still be as succinct as
reasonably possible.

---

Great work. Now, be a little less succinct, and do some more explanations. For
example, you weren't clear about which things were framework supported vs
helpers that live in a project as boilerplate (like NewLoader). I think this is
a little too sparse and lacking details and explanations. It should still be
LLM-targeted.

---

Again, good work. I think it would be much more clear if you basically just
walked through an application (like what the bootstrap creates for you), showing
the filepath and the contents, and explaining each part, and then making sure
you fill in any gaps of available APIs that aren't in the default bootstrapped
project. What do you think? The goal here is to have a document that literally,
with nothing else, an LLM can read and know the entire framework without any
other references and how to set up a full project (in lieu of the bootstrapper).
I don't think we have hit that mark yet.

---

```go
	repoconcat.Concat(repoconcat.Config{
		Output: "__LLM_CONCAT.local/__CURRENT_VERSION.txt",
		Include: []string{
			"wave",
			"internal/framework",
			"kit/matcher",
			"kit/mux",
			"kit/response",
			"kit/headels",
			"bootstrap",
			"internal/site/backend/assets/markdown/docs/kit/matcher.md",
			"internal/site/backend/assets/markdown/docs/kit/mux.md",
			"internal/site/backend/assets/markdown/docs/kit/response.md",
			"internal/site/backend/assets/markdown/docs/kit/headels.md",
		},
		Exclude: []string{
			"wave/internal/configschema",
			// "**/_typescript",
			"internal/framework/_typescript/create",
			"**/*.test.ts",
			"**/*.bench.ts",
			"**/*_test.go",
			"**/bench.txt",
		},
	})
```
