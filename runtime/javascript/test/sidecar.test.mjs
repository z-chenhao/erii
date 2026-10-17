import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createInterface } from 'node:readline';

async function fixture(t, host = async () => ({value: 'ok'})) {
 const p=spawn(process.execPath,[new URL('../src/sidecar.mjs',import.meta.url).pathname], {env:{PATH:'/usr/bin:/bin',TZ:'UTC'}});
 let stderr='';p.stderr.on('data',c=>{stderr+=c});
 const pending=new Map(), events=[];let next=0;
 const send=x=>p.stdin.write(JSON.stringify({jsonrpc:'2.0',...x})+'\n');
 const closed=new Promise(resolve=>p.once('exit',resolve));
 const lines=createInterface({input:p.stdout});
 lines.on('line',async line=>{
  const m=JSON.parse(line);
  if(m.method){
   events.push(m);
   try {send({id:m.id,result:m.method==='execution.event'?{}:await host(m)});}
   catch {send({id:m.id,result:{error:{code:'PERMISSION_DENIED'}}});}
  } else {const f=pending.get(m.id);pending.delete(m.id);f?.(m);}
 });
 p.once('exit',()=>{for(const f of pending.values()) f({error:{message:'sidecar exited: '+stderr}})});
 t.after(async()=>{p.stdin.end();p.kill();await closed;lines.close()});
 const rpc=(method,params)=>new Promise(resolve=>{const id=++next;pending.set(id,resolve);send({id,method,params})});
 const identity=(await rpc('runtime.initialize',{protocolVersion:'1',runtimeVersion:'erii-js-v2',preludeVersion:'erii-prelude-v2'})).result;assert.ok(identity,stderr);
 let execution=0;
 async function run(source, tools=[], options={}){
  const req={protocolVersion:'1',runtimeVersion:'erii-js-v2',preludeVersion:'erii-prelude-v2',executionId:'execution-'+ ++execution,
   runtimeHash:identity.runtimeHash,preludeHash:identity.preludeHash,claimToken:'claim',executionEpoch:1,providerCallId:'provider-call',parentInvocationId:'invocation-'+execution,runId:'run',kind:'worker',source,sourceHash:createHash('sha256').update(source).digest('hex'),tools,
   limits:{timeoutMs:3000,memoryBytes:32<<20,outputBytes:65536,pendingCalls:128,interruptBudget:10000},...options};
  return (await rpc('execution.start',req));
 }
 return {run,events,rpc};
}
const echo={name:'builtin__text__echo',description:'echo'};
test('real QuickJS loops, promises, filtering and canonical tools',async t=>{
 const f=await fixture(t,async m=>({value:m.params.arguments}));
 const result=await f.run('const values = await Promise.all([1,2,3].map(n=>tools.builtin__text__echo({n}))); console.log("HIDDEN"); text(values); return 6;',[echo]);
 assert.equal(result.result?.kind,'completed',JSON.stringify(result));
 assert.deepEqual(result.result.output,[{type:'text',text:'[{"n":1},{"n":2},{"n":3}]'},{type:'return',value:6}]);
});
test('forbidden globals and dynamic escape paths',async t=>{
 const f=await fixture(t);
 const r=await f.run('return [typeof process,typeof fetch,typeof require,typeof setTimeout,typeof Date,typeof Math.random,Function("return typeof Date")(),typeof WebAssembly];');
 assert.deepEqual(r.result?.output,[{type:'return',value:Array(8).fill('undefined')}]);
});
test('errors are catchable; allSettled retains ordered outcomes',async t=>{
 const f=await fixture(t,async m=>m.params.arguments.fail?{error:{code:'APPROVAL_DENIED'}}:{value:2});
 const r=await f.run('const r = await Promise.allSettled([tools.builtin__text__echo({fail:true}),tools.builtin__text__echo({})]); return r.map(x=> x.status === "fulfilled" ? x.value : x.reason.code);',[echo]);
 assert.deepEqual(r.result?.output,[{type:'return',value:['APPROVAL_DENIED',2]}]);
});
test('Store is synchronous and persists even if the script throws',async t=>{
 const values=new Map();
 const f=await fixture(t,async m=>{
  if(m.method==='host.store_put'){values.set(m.params.arguments.key,m.params.arguments.value);return {value:null};}
  return {value:values.get(m.params.arguments.key)};
 });
 const r=await f.run('store("x",{count:4}); text(load("x")); throw new Error("failure");');
 assert.equal(r.result?.kind,'failed');assert.deepEqual(values.get('x'),{count:4});
 assert.deepEqual(r.result.output,[{type:'text',text:'{"count":4}'}]);
});
test('memory, CPU and output are bounded',async t=>{
 const f=await fixture(t);
 for(const source of ['while(true){}','while(true) await null;','let x=[];while(true)x.push(new Array(10000).fill("xxxxxxxx"));','text("x".repeat(100000));']){
  const r=await f.run(source);
  assert.equal(r.result?.kind,'failed',JSON.stringify(r));
 }
});
test('Main hides delegation and rejects forged exposure',async t=>{
 const f=await fixture(t);
 const r=await f.run('return typeof agent;',[],{kind:'main'});
 assert.deepEqual(r.result?.output,[{type:'return',value:'undefined'}]);
 assert.ok((await f.run('return 1;',[{name:'builtin__agent__run'}],{kind:'main'})).error);
});

test('canonical names survive alias collisions', async t => {
 const f=await fixture(t,async m=>({value:m.params.toolName}));
 const tools=[{name:'a-b'},{name:'a.b'},{name:'x-y'},{name:'x_y'}];
 const r=await f.run('return [await tools["a-b"]({}),await tools["a.b"]({}),await tools.x_y({}),ALL_TOOLS.map(t=>t.name)];',tools);
 assert.deepEqual(r.result?.output,[{type:'return',value:['a-b','a.b','x_y',['a-b','a.b','x-y','x_y']]}]);
});
test('cancellation disposes a waiting VM without waiting for a Host result', async t => {
 let reached;const ready=new Promise(resolve=>{reached=resolve});
 const f=await fixture(t,async m=>{reached();return new Promise(()=>{});});
 const running=f.run('await tools.builtin__text__echo({});',[echo]);
 await ready;
 const cancel=await f.rpc('execution.cancel',{executionId:'execution-1'});
 assert.ok(cancel.result);
 assert.equal((await running).result?.kind,'cancelled');
});
test('unawaited calls are registered and their values remain internal',async t=>{
 const calls=[];
 const f=await fixture(t,async m=>{calls.push(m.params);return {suspended:true};});
 const r=await f.run('tools.builtin__text__echo({text:"accepted"});return "done";',[echo]);
 assert.equal(calls.length,1);assert.deepEqual(r.result?.output,[{type:'return',value:'done'}]);
});
test('fresh cells do not retain globals',async t=>{
 const f=await fixture(t);
 assert.equal((await f.run('globalThis.privateValue = 7; return 1;')).result?.kind,'completed');
 assert.deepEqual((await f.run('return typeof privateValue;')).result?.output,[{type:'return',value:'undefined'}]);
});
test('unhandled Host errors preserve only their normalized code',async t=>{
 const f=await fixture(t,async()=>({error:{code:'APPROVAL_DENIED',message:'APPROVAL_DENIED',retryable:false}}));
 const r=await f.run('await tools.builtin__text__echo({});',[echo]);
 assert.equal(r.result?.error?.code,'APPROVAL_DENIED');
 assert.ok(!JSON.stringify(r.result).includes('stack'));
});
test('dynamic imports cannot access modules',async t=>{
 const f=await fixture(t);
 const r=await f.run('try { await import("node:fs"); return "unsafe"; } catch { return "blocked"; }');
 assert.deepEqual(r.result?.output,[{type:'return',value:'blocked'}]);
});
test('Computer bridge tokens are private to bound Surface facades',async t=>{
 const f=await fixture(t,async m=>({value:m.params.toolName==='builtin__computer__bind'?{handle_token:'PRIVATE_LEASE_TOKEN'}:{text:'observed'}}));
 const tools=['discover','bind','invoke'].map(op=>({name:'builtin__computer__'+op}));
 const r=await f.run('const app = await cua.getApp("notes"); let blocked=false;try {store("lease",{app});} catch{blocked=true;} let raw=false;try {await tools.builtin__computer__bind({});} catch{raw=true;} return {blocked, raw, inventory:ALL_TOOLS.length, state:await app.observe()};',tools);
 assert.deepEqual(r.result?.output,[{type:'return',value:{blocked:true,raw:true,inventory:0,state:{text:'observed'}}}]);
 assert.ok(!JSON.stringify(r.result).includes('PRIVATE_LEASE_TOKEN'));
});

test('pending-call limit disposes the VM even when the Host stops replying',async t=>{
 const f=await fixture(t,async()=>new Promise(()=>{}));
 const r=await f.run('for(let n=0;n<129;n++) tools.builtin__text__echo({}); await new Promise(()=>{});',[echo]);
 assert.equal(r.result?.kind,'failed');
 assert.equal(r.result?.error?.code,'RESOURCE_LIMIT');
});

test('suspension registers every issued parallel call before disposing the VM',async t=>{
 const calls=[];
 const f=await fixture(t,async m=>{calls.push(m.params);return {suspended:true};});
 const r=await f.run('const first=tools.builtin__text__echo({n:0});let x=0;for(let i=0;i<1000000;i++)x+=i;await Promise.all([first,...Array.from({length:19},(_,n)=>tools.builtin__text__echo({n:n+1}))]);',[echo]);
 assert.equal(r.result?.kind,'suspended');
 assert.equal(calls.length,20);
 assert.equal(r.result?.pendingCallIds.length,20);
});

test('replay restores durable delivery order despite inverted transport replies',async t=>{
 const f=await fixture(t,async m=>{
  if(m.params.arguments.n===1){await new Promise(resolve=>setTimeout(resolve,20));return {value:'one',deliverySeq:1};}
  return {value:'two',deliverySeq:2};
 });
 const r=await f.run('return await Promise.race([tools.builtin__text__echo({n:1}),tools.builtin__text__echo({n:2})]);',[echo]);
 assert.deepEqual(r.result?.output,[{type:'return',value:'one'}]);
});
test('trusted source positions and dynamic ordinals produce stable scoped call keys',async t=>{
 const keys=[];
 const f=await fixture(t,async m=>{keys.push(m.params.callKey);return {value:1};});
 const source='for(let i=0;i<2;i++) await tools.builtin__text__echo({});\nawait tools.builtin__text__echo({});';
 assert.equal((await f.run(source,[echo])).result?.kind,'completed');
 assert.equal((await f.run(source,[echo])).result?.kind,'completed');
 assert.equal(new Set(keys.slice(0,3)).size,3);
 assert.deepEqual(keys.slice(0,3),keys.slice(3));
 assert.ok(keys.every(k=>/^[a-f0-9]{64}$/.test(k)));
});
test('replay drift fails the cell even if script catches Tool errors',async t=>{
 const f=await fixture(t,async()=>({error:{code:'NON_DETERMINISTIC_REPLAY'}}));
 const r=await f.run('try{await tools.builtin__text__echo({});}catch{} return "unsafe success";',[echo]);
 assert.equal(r.result?.kind,'failed');assert.equal(r.result?.error.code,'NON_DETERMINISTIC_REPLAY');
});
