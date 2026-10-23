import assert from "node:assert/strict";
import test from "node:test";
import { importPluginFromSandbox, withPluginSandbox } from "./plugin-sandbox.mjs";

test("Pi propagates live provider/model through registration, saves, passive capture and compaction", async () => {
  const originalFetch = globalThis.fetch;
  const originalEnv = Object.fromEntries(["ENGRAM_URL","ENGRAM_PROJECT","ENGRAM_AUTHOR"].map(key=>[key,process.env[key]]));
  process.env.ENGRAM_URL="http://127.0.0.1:17437";
  process.env.ENGRAM_AUTHOR="launch/stale";
  delete process.env.ENGRAM_PROJECT;
  const calls=[];
  globalThis.fetch=async(url,options={})=>{
    const path=new URL(url).pathname;
    const body=options.body?JSON.parse(options.body):undefined;
    calls.push({path,body});
    if(path==="/project/current")return new Response('{"project":"test"}');
    if(path==="/sessions")return new Response(JSON.stringify({id:body.id,status:"created"}),{status:201});
    if(path==="/health")return new Response('{"capabilities":{"root_session_resume":true}}');
    return new Response('{"id":1,"status":"saved"}');
  };
  try {
    await withPluginSandbox("engram-pi-author-",async({sandbox})=>{
      const tools=new Map(),events=new Map();
      (await importPluginFromSandbox(sandbox))({registerTool:t=>tools.set(t.name,t),on:(name,fn)=>events.set(name,fn),appendEntry(){}});
      const ctx={cwd:sandbox,model:{provider:"router",id:"router/model-a"},sessionManager:{getSessionId:()=>"host-author"},ui:{setStatus(){}}};
      await events.get("session_start")({},ctx);
      for(const [tool,params] of [["mem_save",{title:"Author",content:"Durable knowledge"}],["mem_session_summary",{content:"Durable summary"}],["mem_capture_passive",{content:"## Key Learnings:\n- Durable passive knowledge."}]]) {
        const result=await tools.get(tool).execute("author",params,undefined,undefined,ctx);
        assert.notEqual(result.isError,true,JSON.stringify(result));
      }
      assert.ok(calls.some(({path,body})=>path==="/sessions"&&body.author==="pi/router/model-a"));
      for(const {path,body} of calls.filter(({path})=>path==="/observations"||path==="/observations/passive"))assert.equal(body.author,"pi/router/model-a",path);
      ctx.model.id="model-b";
      const result=await tools.get("mem_save").execute("next",{title:"Switched",content:"New model knowledge"},undefined,undefined,ctx);
      assert.notEqual(result.isError,true,JSON.stringify(result));
      assert.equal(calls.filter(({path})=>path==="/observations").at(-1).body.author,"pi/router/model-b");
      await events.get("session_compact")({compactionEntry:{summary:"Durable compaction summary"}},ctx);
      assert.equal(calls.filter(({path})=>path==="/observations").at(-1).body.author,"pi/router/model-b");
    });
  } finally {
    globalThis.fetch=originalFetch;
    for(const [key,value]of Object.entries(originalEnv)){if(value===undefined)delete process.env[key];else process.env[key]=value;}
  }
});
