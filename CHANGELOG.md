# Changelog

## 1.0.0 (2026-10-30)


### Features

* add command with strict validation ([337813a](https://github.com/R0b-1ns/cairn/commit/337813ae60178e539326784e1443f728247c4cc0))
* amend command ([f6e3827](https://github.com/R0b-1ns/cairn/commit/f6e3827a70f2c55562fc5482f7ae198f33946852))
* atomic, idempotent import in the store ([c716fb8](https://github.com/R0b-1ns/cairn/commit/c716fb86fc804580fc8ff835ac7427ee605e78a0))
* brief command and output sanitizer ([f2d8487](https://github.com/R0b-1ns/cairn/commit/f2d84873ac0324d5891e793a03a8b54f780b9648))
* computed note status and entry state ([fac6264](https://github.com/R0b-1ns/cairn/commit/fac6264a0697a6c159237d50c654e941a6a8ba89))
* default repo set at init ([84ab55b](https://github.com/R0b-1ns/cairn/commit/84ab55bb546de9625fc5920970dabae480a6f93e))
* domain model, validation and secret detection ([000be2d](https://github.com/R0b-1ns/cairn/commit/000be2d43e0c9b3df700413ef60c27a785a62b23))
* entry listing and subject queries ([68fd174](https://github.com/R0b-1ns/cairn/commit/68fd174a5314211b5a03442d7d5d10d5e5fe44cd))
* export and import ([bf7bc39](https://github.com/R0b-1ns/cairn/commit/bf7bc390ba10e5ddada7a3a089a46ed477c4d520))
* init, context and project registry commands ([edf6bd4](https://github.com/R0b-1ns/cairn/commit/edf6bd4d2d28080e0ff5657b56511444f232e7ef))
* initial implementation of cairn ([f1755b9](https://github.com/R0b-1ns/cairn/commit/f1755b90b6921fc67b12341c750b9bf4a6b183f0))
* machine registry and project lookup ([e5ae5f5](https://github.com/R0b-1ns/cairn/commit/e5ae5f55b94a64e43d3650be9417fb15581608fa))
* note headers with current status ([ec743d6](https://github.com/R0b-1ns/cairn/commit/ec743d6a4f96414daf6d18ae4eebdba7367dc682))
* search and check commands ([e43a0b8](https://github.com/R0b-1ns/cairn/commit/e43a0b834d55be10b29395bd86b8ba59f0f5a9bd))
* show, list and recent commands ([eac2d90](https://github.com/R0b-1ns/cairn/commit/eac2d907ed3cefe4f2970ce461735c10947c7036))
* sqlite store with immutability triggers ([afa7e8f](https://github.com/R0b-1ns/cairn/commit/afa7e8f0cfe8f8232e6ddc724300d3d591392330))


### Bug Fixes

* block INSERT OR REPLACE from rewriting immutable rows ([0ea7246](https://github.com/R0b-1ns/cairn/commit/0ea7246d74f3426811ed5f36fc95dffc7adc4d6d))
* end-of-flags handling, detailed help, brief JSON and project move checks ([c677d0f](https://github.com/R0b-1ns/cairn/commit/c677d0ff2a7b4d1aa1011bf531bf7f90a6ff5506))
* harden input against invisible text, forged timestamps and secret leaks ([f703cfc](https://github.com/R0b-1ns/cairn/commit/f703cfc636688977b469e6c7e04df7e37f751cea))
* never follow symlinks when exporting to a file ([894fab9](https://github.com/R0b-1ns/cairn/commit/894fab92431538905be42eee27d3d1b8b8f9c9e9))
* refuse untrusted stores and check symlinks when selecting by name ([4a261a8](https://github.com/R0b-1ns/cairn/commit/4a261a88827cdb8a397b587eaabd0907edbe90a2))
* root trusts only its own or the sudo user's stores ([fca56cf](https://github.com/R0b-1ns/cairn/commit/fca56cf36aaa9fa2a23c56294846e035e70ef08b))
* sanitize all terminal output at one choke point ([98e2dd3](https://github.com/R0b-1ns/cairn/commit/98e2dd35187512d5443ad2d6e989669047c10006))
* verify the whole schema when opening a database ([93a198c](https://github.com/R0b-1ns/cairn/commit/93a198c48c3aa067a38e792b2af5d513d03ecdd1))
