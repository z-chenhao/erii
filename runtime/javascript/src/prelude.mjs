import { PRELUDE_SOURCE as PI_PRELUDE } from './vendor/prelude.mjs';

// The pinned upstream prelude supplies Promise jobs, serialization, output
// bounds and intrinsic lockdown. Erii changes only its public Host contract.
function replaceOnce(source, before, after) {
  if (source.split(before).length !== 2) throw new Error('incompatible pinned Pi prelude');
  return source.replace(before, after);
}
let source = replaceOnce(PI_PRELUDE, '"use strict";', `"use strict";
  // Remove these before upstream recursively freezes the intrinsic graph.
  delete globalThis.Date;
  delete Math.random;
  delete globalThis.WeakRef;
  delete globalThis.FinalizationRegistry;
`);
source = replaceOnce(source, 'allTools.push(Object.freeze({ name: jsName, description }));',
  'if (!["builtin__computer__discover", "builtin__computer__bind", "builtin__computer__invoke"].includes(name)) allTools.push(Object.freeze({ name, description }));');
source = replaceOnce(source, 'const toolsProxy = guard(\n\t\ttools,', `const publicTools = Object.create(null);
  for (const key of Object.keys(tools)) {
    if (!["builtin__computer__discover", "builtin__computer__bind", "builtin__computer__invoke"].includes(key)) publicTools[key] = tools[key];
  }
  Object.freeze(publicTools);
  const toolsProxy = guard(
    publicTools,`);
source = replaceOnce(source, 'bridge(kind, id, name, json);',
  'bridge(kind, id, name, json, new ErrorCtor().stack);');
source = replaceOnce(source, '// The first tool wins when two names normalize to the same identifier.',
  '// Go/Node validate the complete alias map before VM admission.');
const start = source.indexOf('\t// key -> JSON text.');
const end = source.indexOf('\n\tlet outputChars', start);
if (start < 0 || end < 0) throw new Error('incompatible pinned Pi Store');
source = source.slice(0, start) + `
  const handles = new WeakSet();
  function sync(method, value) {
    const reply = parse(bridge("sync", method, stringify(value), new ErrorCtor().stack));
    if (reply.error) { const e = new ErrorCtor(reply.error.code); e.code = reply.error.code; throw e; }
    return reply.value;
  }
  function rejectHandles(value, seen = new Set()) {
    if (value === null || typeof value !== "object") return;
    if (handles.has(value) || "handle_token" in value || "handleToken" in value) throw new TypeErrorCtor("Surface handles cannot be stored");
    if (seen.has(value)) throw new TypeErrorCtor("Cyclic Store value");
    seen.add(value);
    for (const key of Object.keys(value)) rejectHandles(value[key], seen);
    seen.delete(value);
  }
  function store(key, value) {
    if (typeof key !== "string" || !key || key.length > 256) throw new TypeErrorCtor("Invalid Store key");
    rejectHandles(value);
    // Serialization fails for functions/undefined rather than silently saving them.
    const json = stringify(value);
    if (json === undefined || json.length > 262144) throw new RangeErrorCtor("Store value limit");
    return sync("host.store_put", {key, value: parse(json)});
  }
  function load(key) {
    if (typeof key !== "string" || !key || key.length > 256) throw new TypeErrorCtor("Invalid Store key");
    return sync("host.store_get", {key});
  }
  function serializeWrites() { return "[]"; }
  Object.defineProperty(globalThis, "store", {value: store, enumerable: true});
  Object.defineProperty(globalThis, "load", {value: load, enumerable: true});

  const invoke = (name, args) => tools[name](args);
  const cuaDiscover = (method, args) => invoke("builtin__computer__discover", {method, args: args === undefined ? null : args});
  const cuaInvoke = (token, method, args) => invoke("builtin__computer__invoke", {handle_token: token, method, args});
  function surface(value) {
    const token = value.handle_token;
    if (typeof token !== "string") throw new TypeErrorCtor("Invalid Surface binding");
    const target = Object.freeze(Object.create(null));
    const proxy = new Proxy(target, {get(_, key) {
      if (key === "then" || key === "toJSON") return undefined;
      if (typeof key !== "string") return undefined;
      if (key === "capabilities") return Object.freeze({
        list: () => cuaInvoke(token, "capabilities.list", []),
        get: (name) => cuaInvoke(token, "capabilities.get", [name]),
        documentation: (name) => cuaInvoke(token, "capabilities.documentation", [name])
      });
      return (...args) => cuaInvoke(token, key, args);
    }});
    handles.add(proxy);
    return proxy;
  }
  const cua = Object.create(null);
  for (const method of ["getState", "listApps", "listWindows", "listBrowsers", "listTabs"])
    cua[method] = (args) => cuaDiscover(method, args);
  for (const method of ["getApp", "getBrowser", "getTab", "createBrowserTab"])
    cua[method] = (...args) => invoke("builtin__computer__bind", {method, args}).then(surface);
  Object.defineProperty(globalThis, "cua", {value: Object.freeze(cua), enumerable: true});
  if (Object.hasOwn(tools, "builtin__agent__run"))
    Object.defineProperty(globalThis, "agent", {value: Object.freeze({run: (request) => invoke("builtin__agent__run", request)}), enumerable: true});
` + source.slice(end);
// Console output remains internal. Only explicit text/image/return is exposed.
source = replaceOnce(source, 'output("console", args.map(format).join(" "));', 'void args;');
export const PRELUDE_SOURCE = source;
