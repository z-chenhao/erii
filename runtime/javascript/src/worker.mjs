import { parentPort, workerData } from 'node:worker_threads';
import { startVm } from './vendor/vm.mjs';

const interrupt = new Int32Array(workerData.interrupt);
let polls = 0;
const post = (message) => parentPort.postMessage(message);
try {
  const session = await startVm({
    data: workerData,
    wasm: workerData.wasm,
    post,
    shouldInterrupt: () => Atomics.load(interrupt, 0) !== 0 || ++polls > workerData.interruptBudget,
    interruptBudget: workerData.interruptBudget,
    sync(method, args, position) {
      const memory = new SharedArrayBuffer(1048584);
      const control = new Int32Array(memory, 0, 2);
      post({type: 'sync', method, args, position, memory});
      if (Atomics.wait(control, 0, 0, workerData.timeoutMs) === 'timed-out')
        throw new Error('TIMEOUT');
      const size = Atomics.load(control, 1);
      if (size < 0 || size > 1048576) throw new Error('RESOURCE_LIMIT');
      return new TextDecoder().decode(new Uint8Array(memory, 8, size));
    },
  });
  parentPort.on('message', (message) => {
    if (message.type === 'result') session.deliver(message);
  });
} catch {
  post({type: 'crash', message: 'JavaScript runtime failed'});
}
