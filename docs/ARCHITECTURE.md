# Architecture

HUD will turn RED if the template performs a DB mutation without the mandatory audit wrapper/marker (Naked Mutation).

The CLI exposes two surfaces:
- CLI commands (setup flow: `setup`, `target`, `classify`, `detect-agents`, `connect-agent`, `validate-agent`; actions: `scan`, `verify`, `watch`, `shadow`, `fix`, `fix-loop`, `support-bundle`, `rollback`, `doctor`)
- MCP server over stdio (`gres-b2b mcp serve`, JSON-RPC 2.0, Content-Length framing)

Wizard UI is a thin wrapper over the same CLI flow steps.

Report artifacts are written under `.b2b/` and used by the HUD.

Install guide: `docs/INSTALL.md`
