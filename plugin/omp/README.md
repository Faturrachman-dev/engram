# Oh-My-Pi integration

Add the `engram` entry from `mcp-template.json` to `~/.omp/agent/mcp.json` and
copy `engram.ts` to `~/.omp/agent/extensions/engram.ts`. Use an absolute binary
path if `engram` is not on OMP's PATH. Start `engram serve` and restart OMP.

The MCP server supplies memory tools. The extension binds mutating calls to the
host session and stamps observation writes with the live `omp/provider/model`.
No prompt, transcript, or routine tool output is automatically saved. Model
changes are read from the execution context on every write, not a static config.
If OMP exposes no model, `ENGRAM_AUTHOR` is the fallback. Registration failures
block writes rather than inventing session IDs or provenance.

The server must support resumed sessions. Read-only memory tools do not require
registration. This adapter uses OMP's input-rewriting `tool_call` API (verified
against 18.6.1); Pi has a separate native adapter under `plugin/pi`.

Run adapter tests with `node --test plugin/omp/engram.test.mjs` from the repo root.
