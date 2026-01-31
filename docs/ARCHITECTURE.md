# Architecture

HUD will turn RED if the template performs a DB mutation without the mandatory audit wrapper/marker (Naked Mutation).

The CLI exposes two surfaces:
- CLI commands (`scan`, `verify`, `doctor`, `watch`, `shadow`, `fix`, `support-bundle`, `rollback`)
- MCP server over stdio (`gres-b2b mcp serve`, JSON-RPC 2.0, Content-Length framing)

Report artifacts are written under `.b2b/` and used by the HUD.

Install guide: `docs/INSTALL.md`
