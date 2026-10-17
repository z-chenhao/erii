import { Worker } from 'node:worker_threads';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';

export const VERSION = 'erii-js-v2';
const PRELUDE = 'erii-prelude-v2';
const MAX_MESSAGE = 1048576;
const executions = new Map();
const replies = new Map();
let initialized = false;
let nextID = 0;
let active = 0;
const require = createRequire(import.meta.url);
async function fingerprint(paths) {
 const hash=createHash('sha256');
 for(const path of paths) hash.update(await readFile(new URL(path,import.meta.url)));
 return hash.digest('hex');
}
const runtimeHash=await fingerprint(['sidecar.mjs','worker.mjs','vendor/vm.mjs','../package-lock.json','../node_modules/quickjs-wasi/quickjs.wasm']);
const preludeHash=await fingerprint(['prelude.mjs','vendor/prelude.mjs']);
const wasm = WebAssembly.compile(await readFile(require.resolve('quickjs-wasi/quickjs.wasm')));

function send(value) {
  const json = JSON.stringify(value);
  if (Buffer.byteLength(json) > MAX_MESSAGE) throw new Error('RESOURCE_LIMIT');
  process.stdout.write(json + '\n');
}
function error(id, code) { send({jsonrpc: '2.0', id, error: {code: -32000, message: code}}); }
function request(method, params) {
  const id = 's' + ++nextID;
  return new Promise((resolve, reject) => {
    replies.set(id, {resolve, reject});
    try { send({jsonrpc: '2.0', id, method, params}); }
    catch (e) { replies.delete(id); reject(e); }
  });
}
function envelope(e) {
  return {protocolVersion: '1', runtimeId: VERSION, executionId: e.req.executionId,
    parentInvocationId: e.req.parentInvocationId, runId: e.req.runId,
    claimToken: e.req.claimToken, executionEpoch: e.req.executionEpoch,
    sequence: ++e.sequence, occurredAt: new Date().toISOString()};
}
function event(e, kind, fields = {}) {
  // Every authoritative event is an RPC request. Resolve only after Go commits.
  e.events = e.events.then(() => request('execution.event', {...envelope(e), kind, ...fields}));
  return e.events;
}
function aliases(specs) {
  const canonical = new Set(specs.map(s => s.name));
  if (canonical.size !== specs.length) throw new Error('INVALID_ARGUMENT');
  const counts = new Map();
  const names = specs.map(s => s.name.replace(/[^a-zA-Z0-9_$]/g, '_').replace(/^(?=[0-9])/, '_'));
  for (const name of names) counts.set(name, (counts.get(name) || 0) + 1);
  return specs.map((s, i) => ({...s, jsName: counts.get(names[i]) === 1 && (!canonical.has(names[i]) || names[i] === s.name) ? names[i] : s.name}));
}
function valid(req) {
  return req && req.runtimeVersion === VERSION && req.preludeVersion === PRELUDE &&
    req.protocolVersion === '1' && req.runtimeHash === runtimeHash && req.preludeHash === preludeHash && typeof req.executionId === 'string' && req.executionId &&
    typeof req.parentInvocationId === 'string' && req.parentInvocationId && typeof req.runId === 'string' && req.runId &&
    typeof req.claimToken === 'string' && req.claimToken && Number.isSafeInteger(req.executionEpoch) && req.executionEpoch > 0 && typeof req.providerCallId === 'string' && req.providerCallId &&
    ['main', 'worker'].includes(req.kind) && typeof req.source === 'string' &&
    Buffer.byteLength(req.source) <= 65536 && req.sourceHash === createHash('sha256').update(req.source).digest('hex') &&
    Array.isArray(req.tools) && req.tools.length <= 1024 &&
    req.tools.every(s => typeof s.name === 'string' && s.name.length <= 256 && (req.kind === 'worker' || s.name !== 'builtin__agent__run')) &&
    req.limits && Number.isInteger(req.limits.timeoutMs) && req.limits.timeoutMs > 0 && req.limits.timeoutMs <= 300000 &&
    Number.isInteger(req.limits.memoryBytes) && req.limits.memoryBytes >= 1048576 && req.limits.memoryBytes <= 67108864 &&
    Number.isInteger(req.limits.outputBytes) && req.limits.outputBytes > 0 && req.limits.outputBytes <= 262144 &&
    Number.isInteger(req.limits.pendingCalls) && req.limits.pendingCalls > 0 && req.limits.pendingCalls <= 128 &&
    Number.isInteger(req.limits.interruptBudget) && req.limits.interruptBudget > 0 && req.limits.interruptBudget <= 100000;
}
async function stop(e, kind, code) {
  if (e.finished) return;
  e.finished = true;
  e.stopKind = kind;
  clearTimeout(e.timer);
  Atomics.store(new Int32Array(e.interrupt), 0, 1);
  await e.worker?.terminate();
  // Host-accepted calls belong to Go and continue independently of the VM.
  if (kind === 'completed' || kind === 'suspended') {
    let timer;
    const accepted = await Promise.race([
      e.hosts.then(() => true),
      new Promise(resolve => { timer = setTimeout(() => resolve(false), Math.max(1, e.deadlineAt - Date.now())); }),
    ]);
    clearTimeout(timer);
    if (!accepted) {kind = 'failed';code = 'TIMEOUT';e.stopKind = kind;}
  }
  const result = {kind, output: e.output, pendingCallIds: [...e.pending].filter(id => e.posted.has(id)), error: code ? {code} : undefined};
  try {
    await event(e, 'execution_' + kind, {outcome: result, metrics: {activeVMs: active, nodeRssBytes: process.memoryUsage().rss, elapsedMs: Date.now() - (e.deadlineAt - e.req.limits.timeoutMs)}});
    send({jsonrpc: '2.0', id: e.id, result});
  } catch { error(e.id, 'PROVIDER_UNAVAILABLE'); }
  executions.delete(e.req.executionId);
}
function nestedKey(e, message) {
  // Captured by the frozen trusted Prelude, never supplied in Tool arguments.
  const position = String(message.position || '').split('\n').filter(line => line.includes('codemode.js')).join('\n');
  const ordinal = (e.positions.get(position) || 0) + 1;
  e.positions.set(position, ordinal);
  return createHash('sha256').update(JSON.stringify([e.req.providerCallId, e.req.sourceHash, position, ordinal])).digest('hex');
}
async function hostCall(e, message) {
  if (e.finished) return;
  if (e.pending.size >= e.req.limits.pendingCalls) { queueMicrotask(() => {void stop(e, 'failed', 'RESOURCE_LIMIT')}); return; }
  const callId = String(message.id);
  const callKey = nestedKey(e, message);
  if (e.seen.has(callId)) { queueMicrotask(() => {void stop(e, 'failed', 'PERMISSION_DENIED')}); return; }
  e.seen.add(callId);
  e.pending.add(callId);
  const args = message.args === undefined ? null : JSON.parse(message.args);
  const discovery = message.target === 'global';
  if ((!discovery && !e.req.tools.some(t => t.name === message.name)) ||
      (discovery && !['searchTools', 'describeTool', 'describeNamespace'].includes(message.name)))
    { queueMicrotask(() => {void stop(e, 'failed', 'PERMISSION_DENIED')}); return; }
  await event(e, 'call_started', {callId, toolName: message.name});
  if (e.finished && e.stopKind !== 'completed' && e.stopKind !== 'suspended') return;
  e.posted.add(callId);
  const result = await request(discovery ? 'host.discover' : 'host.invoke', {
    ...envelope(e), callId, callKey, toolName: message.name, arguments: args,
  });
  if (result.error?.code === 'NON_DETERMINISTIC_REPLAY') { void stop(e, 'failed', 'NON_DETERMINISTIC_REPLAY'); return; }
  if (result.suspended) {
    e.suspend = true;
    return;
  }
  if (result.deliverySeq) {
    // Serialize the entire delivery, including its durable event ack and VM
    // post. Merely resolving ordered Promises can invert their continuations.
    return new Promise(resolve => {
      e.deliveries.set(result.deliverySeq, {message, result, callId, resolve});
      while (e.deliveries.has(e.nextDelivery)) {
        const next = e.deliveries.get(e.nextDelivery);
        e.deliveries.delete(e.nextDelivery++);
        e.deliveryWork = e.deliveryWork.then(() => deliverResult(e, next.message, next.result, next.callId)).then(next.resolve);
      }
    });
  }
  await deliverResult(e, message, result, callId);
}
async function deliverResult(e, message, result, callId) {
  e.pending.delete(callId);
  await event(e, 'call_finished', {callId});
  if (e.finished) return;
  e.idle = undefined;
  e.worker.postMessage({type: 'result', id: message.id, ok: !result.error,
    payload: result.error ? result.error.code : JSON.stringify(result.value),
    properties: result.error ? JSON.stringify(result.error) : undefined,
    seq: ++e.delivered, now: 0});
}
async function syncCall(e, message) {
  let result;
  try {
    if (!['host.store_put', 'host.store_get'].includes(message.method)) throw new Error('PERMISSION_DENIED');
    result = await request(message.method, {...envelope(e), callKey: nestedKey(e, message), arguments: JSON.parse(message.args)});
  } catch { result = {error: {code: 'PROVIDER_UNAVAILABLE'}}; }
  if (result.error?.code === 'NON_DETERMINISTIC_REPLAY') {
    Atomics.store(new Int32Array(e.interrupt), 0, 1);
    void stop(e, 'failed', 'NON_DETERMINISTIC_REPLAY');
  }
  const bytes = new TextEncoder().encode(JSON.stringify(result));
  const control = new Int32Array(message.memory, 0, 2);
  if (bytes.length > MAX_MESSAGE) Atomics.store(control, 1, -1);
  else { new Uint8Array(message.memory, 8, bytes.length).set(bytes); Atomics.store(control, 1, bytes.length); }
  Atomics.store(control, 0, 1);
  Atomics.notify(control, 0);
}
async function start(id, req) {
  if (!initialized || !valid(req) || executions.has(req.executionId)) return error(id, 'INVALID_ARGUMENT');
  if (active >= 4) return error(id, 'RESOURCE_LIMIT');
  const tools = aliases(req.tools);
  const e = {id, req, deadlineAt: Date.now() + req.limits.timeoutMs, output: [], bytes: 0, pending: new Set(), posted: new Set(), seen: new Set(), sequence: 0, delivered: 0,
    positions: new Map(), deliveries: new Map(), nextDelivery: 1, deliveryWork: Promise.resolve(),
    events: Promise.resolve(), hosts: Promise.resolve(), interrupt: new SharedArrayBuffer(4)};
  executions.set(req.executionId, e);
  await event(e, 'execution_started');
  if (e.finished) return;
  active++;
  e.worker = new Worker(new URL('./worker.mjs', import.meta.url), {workerData: {
    code: req.source, tools, globals: [
      {name: 'searchTools', spread: true}, {name: 'describeTool'}, {name: 'describeNamespace'}],
    store: {}, wasm: await wasm, interrupt: e.interrupt,
    deterministic: {startedAt: 0, seed: 0}, memoryLimitBytes: req.limits.memoryBytes,
    timeoutMs: req.limits.timeoutMs, interruptBudget: req.limits.interruptBudget,
  }});
  e.worker.once('exit', () => { active--; });
  e.timer = setTimeout(() => { void stop(e, 'failed', 'TIMEOUT'); }, req.limits.timeoutMs);
  e.worker.on('error', () => { void stop(e, 'failed', 'PROVIDER_UNAVAILABLE'); });
  e.worker.on('message', (m) => {
    if (e.finished) return;
    if (m.type === 'call') {
      const call = hostCall(e, m).catch(() => {queueMicrotask(() => {void stop(e, 'failed', 'PROVIDER_UNAVAILABLE')})});
      e.hosts = Promise.all([e.hosts, call]);
      void call.then(() => { if (e.suspend && !e.finished && e.idle === e.delivered) void stop(e, 'suspended'); });
    } else if (m.type === 'idle') {
      e.idle = m.seq;
      if (e.suspend && e.idle === e.delivered) void stop(e, 'suspended');
    } else if (m.type === 'sync') {
      const call = syncCall(e, m);
      e.hosts = Promise.all([e.hosts, call]);
    } else if (m.type === 'output') {
      const size = Buffer.byteLength(JSON.stringify(m.item));
      if ((e.bytes += size) > req.limits.outputBytes || e.output.length >= 256) void stop(e, 'failed', 'RESOURCE_LIMIT');
      else { e.output.push(m.item); void event(e, 'output_appended', {output: m.item}).catch(() => stop(e, 'failed', 'PROVIDER_UNAVAILABLE')); }
    } else if (m.type === 'done') {
      if (m.ok && m.value !== undefined) {
        const item = {type: 'return', value: JSON.parse(m.value)};
        if ((e.bytes += Buffer.byteLength(JSON.stringify(item))) > req.limits.outputBytes) return void stop(e, 'failed', 'RESOURCE_LIMIT');
        e.output.push(item);
      }
      const codes = new Set(['INVALID_ARGUMENT','NOT_FOUND','PERMISSION_DENIED','APPROVAL_DENIED','TIMEOUT','CANCELLED','PROVIDER_UNAVAILABLE','RESOURCE_LIMIT','UNKNOWN_SIDE_EFFECT','NON_DETERMINISTIC_REPLAY']);
      let code;
      if (!m.ok) {
        let detail;
        try { detail = JSON.parse(m.error); } catch { detail = {}; }
        code = codes.has(detail.message) ? detail.message : /out of memory/i.test(detail.message || '') ? 'RESOURCE_LIMIT' : 'INVALID_ARGUMENT';
      }
      void stop(e, m.ok ? 'completed' : 'failed', code);
    } else if (m.type === 'crash') void stop(e, 'failed', 'RESOURCE_LIMIT');
  });
}
async function receive(message) {
  if (message.jsonrpc !== '2.0') throw new Error('INVALID_ARGUMENT');
  if (!message.method) {
    const pending = replies.get(message.id);
    if (!pending) throw new Error('PERMISSION_DENIED');
    replies.delete(message.id);
    if (message.error) pending.reject(new Error('PROVIDER_UNAVAILABLE'));
    else pending.resolve(message.result);
    return;
  }
  const p = message.params;
  switch (message.method) {
    case 'runtime.initialize':
      if (process.versions.node !== '24.21.0' || initialized || p?.protocolVersion !== '1' || p?.runtimeVersion !== VERSION || p?.preludeVersion !== PRELUDE) return error(message.id, 'INVALID_ARGUMENT');
      initialized = true;
      send({jsonrpc: '2.0', id: message.id, result: {protocolVersion: '1', runtimeVersion: VERSION, preludeVersion: PRELUDE, runtimeHash, preludeHash}});
      break;
    case 'execution.start': await start(message.id, p); break;
    case 'execution.cancel': case 'execution.dispose': {
      const e = executions.get(p.executionId);
      if (e) await stop(e, 'cancelled', 'CANCELLED');
      send({jsonrpc: '2.0', id: message.id, result: {}});
      break;
    }
    case 'runtime.shutdown':
      await Promise.all([...executions.values()].map(e => stop(e, 'cancelled', 'CANCELLED')));
      send({jsonrpc: '2.0', id: message.id, result: {}});
      process.stdin.destroy();
      break;
    default: error(message.id, 'NOT_FOUND');
  }
}
let buffer = Buffer.alloc(0);
process.stdin.on('data', (chunk) => {
  buffer = Buffer.concat([buffer, chunk]);
  let end;
  while ((end = buffer.indexOf(10)) >= 0) {
    const line = buffer.subarray(0, end);
    buffer = buffer.subarray(end + 1);
    if (line.length > MAX_MESSAGE) return process.exit(2);
    try { const message = JSON.parse(line); void receive(message).catch(() => error(message.id, 'INVALID_ARGUMENT')); }
    catch { return process.exit(2); }
  }
  if (buffer.length > MAX_MESSAGE) process.exit(2);
});
process.stdin.on('end', () => {
  for (const e of executions.values()) { clearTimeout(e.timer); void e.worker?.terminate(); }
});
