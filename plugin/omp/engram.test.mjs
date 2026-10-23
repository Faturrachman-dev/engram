import assert from "node:assert/strict";
import test from "node:test";
import register from "./engram.ts";

test("OMP rewrites author/session from live host context and isolates other tools", async () => {
  const original = globalThis.fetch;
  let handler; register({ on(name, fn) { assert.equal(name, "tool_call"); handler = fn; } });
  const registrations = [];
  globalThis.fetch = async (url, options) => {
    if (url.includes("/project/current")) return new Response('{"project":"test"}');
    registrations.push(JSON.parse(options.body));
    return new Response('{"id":"host","status":"created"}', {status:201});
  };
  try {
    const ctx = { cwd:"/work", model:{provider:"router",id:"router/model-a"}, sessionManager:{getSessionId:()=>"host"} };
    const event = {toolName:"mcp__engram_mem_save",input:{title:"test",author:"forged",session_id:"forged"}};
    assert.deepEqual(await handler(event,ctx),{input:{title:"test",author:"omp/router/model-a",session_id:"host"}});
    assert.equal(registrations[0].author,"omp/router/model-a");
    ctx.model.id="model-b";
    assert.equal((await handler(event,ctx)).input.author,"omp/router/model-b");
    assert.equal(await handler({toolName:"mcp__other_mem_save",input:{}},ctx),undefined);
    assert.equal(await handler({toolName:"mcp__engram_mem_search",input:{}},ctx),undefined);
    globalThis.fetch = async () => { throw new Error("offline"); };
    assert.equal((await handler(event,ctx)).block,true);
  } finally { globalThis.fetch=original; }
});
