import type { ExtensionAPI, ExtensionContext } from "@oh-my-pi/pi-coding-agent";

const writes = new Set([
  "mem_save", "mem_update", "mem_review", "mem_delete", "mem_save_prompt",
  "mem_pin", "mem_unpin", "mem_session_summary", "mem_session_start",
  "mem_session_end", "mem_capture_passive", "mem_merge_projects", "mem_judge", "mem_compare",
]);
const authorTools = new Set(["mem_save", "mem_session_summary", "mem_capture_passive", "mem_session_start"]);

function authorFor(ctx: ExtensionContext): string | undefined {
  const model = ctx.model?.id?.trim();
  const provider = ctx.model?.provider?.trim();
  if (!model) return process.env.ENGRAM_AUTHOR?.trim() || undefined;
  return `omp/${provider && !model.startsWith(`${provider}/`) ? `${provider}/` : ""}${model}`;
}

export default function registerEngram(pi: ExtensionAPI) {
  pi.on("tool_call", async (event, ctx) => {
    const prefix = ["mcp__engram__", "mcp__engram_"].find((value) => event.toolName.startsWith(value));
    if (!prefix) return;
    const tool = event.toolName.slice(prefix.length);
    if (!writes.has(tool)) return;
    const hostID = ctx.sessionManager.getSessionId();
    if (!hostID) return { block: true, reason: "Engram requires an authoritative OMP session ID." };
    const base = (process.env.ENGRAM_URL || `http://127.0.0.1:${process.env.ENGRAM_PORT || "7437"}`).replace(/\/$/, "");
    const signal = AbortSignal.timeout(4000);
    const author = authorFor(ctx);
    try {
      const current = await fetch(`${base}/project/current?cwd=${encodeURIComponent(ctx.cwd)}`, { signal });
      if (!current.ok) throw new Error("project resolution failed");
      const project = await current.json() as { project?: string; available_projects?: string[]; error_hint?: string };
      if (project.error_hint || !project.project || (project.available_projects && project.available_projects.length > 1)) {
        throw new Error("select an unambiguous Engram project before writing");
      }
      const response = await fetch(`${base}/sessions`, {
        method: "POST", signal, headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: hostID, project: project.project, directory: ctx.cwd, ownership_mode: "shared", resume: tool !== "mem_session_end", author }),
      });
      if (response.status !== 201) throw new Error("session registration failed");
      const ack = await response.json() as { id?: string; status?: string };
      const continuation = typeof ack.id === "string" && ack.id.startsWith(`${hostID}:resume:`)
        && /^[0-9]+$/.test(ack.id.slice(`${hostID}:resume:`.length));
      if (ack.status !== "created" || !(ack.id === hostID || continuation)) throw new Error("session acknowledgement invalid");
      const input: Record<string, unknown> = { ...event.input };
      input[tool === "mem_session_start" || tool === "mem_session_end" ? "id" : "session_id"] = ack.id;
      if (authorTools.has(tool)) {
        delete input.author;
        if (author) input.author = author;
      }
      return { input };
    } catch {
      // Never log tool arguments, agent IDs, or raw server responses.
      return { block: true, reason: "Engram could not confirm the OMP session/project. Check engram serve and project configuration; no memory write was dispatched." };
    }
  });
}
