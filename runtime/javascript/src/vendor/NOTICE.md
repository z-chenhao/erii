`prelude.mjs` and `vm.mjs` originate from the MIT-licensed
`@earendil-works/pi-codemode` npm release 1.3.0:
https://github.com/earendil-works/pi/tree/v1.3.0/packages/codemode

Published tarball integrity:
sha512-2G9tmbsPhgzwfz7cmzJpbLSLqOx3eu22Gbzm+Wc3HqeZMjLQS4F3KfAabozVhv7uEjq3k2O1dZTHmhhWr2xbfQ==

`prelude.mjs` preserves the upstream implementation. Erii's `../prelude.mjs`
applies checked adaptations for synchronous durable Store, canonical metadata,
CUA/Agent facades, explicit-only output, and prohibited time/random globals.
`vm.mjs` changes the prelude import and adds a primitive-only synchronous Host
callback plus a primitive source-stack argument captured by the trusted Prelude. The Node worker owns CPU interruption, cancellation and disposal.
There is no alternate JavaScript interpreter or model execution path.

The MIT notice is retained in LICENSE. QuickJS-WASI 3.6.2 is separately pinned
in package-lock.json and includes its upstream license in its distribution.
