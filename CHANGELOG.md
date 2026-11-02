# Changelog

## [2.0.0](https://github.com/R0b-1ns/cairn/compare/v1.3.0...v2.0.0) (2026-11-02)


### ⚠ BREAKING CHANGES

* databases migrate automatically to schema version 5 on first open (a .v4.bak or .v3.bak copy is kept next to them) and older binaries refuse them.

### Features

* **cli:** brief and export include local children ([8da4f4e](https://github.com/R0b-1ns/cairn/commit/8da4f4e49d26227024f33c6e354ed93d57ce5abc))
* **cli:** open a parent's local children for reading ([fd8ab6d](https://github.com/R0b-1ns/cairn/commit/fd8ab6dab00589230d8bd928dd541fa3574250d6))
* **cli:** search, list and show include local children ([fc1a240](https://github.com/R0b-1ns/cairn/commit/fc1a240d0a778141466394071311e08ea76f528a))
* **init:** ignore database backups in git ([63cdbed](https://github.com/R0b-1ns/cairn/commit/63cdbed3a366eebfd508942bc3f3a980393f2ce3))
* **init:** join an existing project as an attached or local child ([ee6960c](https://github.com/R0b-1ns/cairn/commit/ee6960caf9025561e3353c0e116683ef56cd166d))
* let a new repo join an existing project (schema version 5) ([c15bc9a](https://github.com/R0b-1ns/cairn/commit/c15bc9a280e600c8a5209a8197edd42829445647))
* **locate:** follow an attached child's pointer to its parent ([933ed38](https://github.com/R0b-1ns/cairn/commit/933ed3820de258f98234fbd568af70830c03383f))
* **project:** link and unlink children ([940f816](https://github.com/R0b-1ns/cairn/commit/940f81624294867e5c4b748c2224242f221a0379))
* **project:** record project ids and report children without a path ([77f006a](https://github.com/R0b-1ns/cairn/commit/77f006abcec7a5d2e01d959b5746d47e072e4270))
* **registry:** remember project ids and the paths of children ([56933e8](https://github.com/R0b-1ns/cairn/commit/56933e89d0ab7613a33b4911a5e1f697de46bba7))
* **store:** schema version 5 with an append-only link_events table ([78937d2](https://github.com/R0b-1ns/cairn/commit/78937d239a33de576b20e9e2df5f6f4385ab8017))
* **view:** include local children ([81e4534](https://github.com/R0b-1ns/cairn/commit/81e4534efd973371b3b8ea89aafaeb25412e7cad))


### Bug Fixes

* correct the out-of-date child warning, export the stored project, harden pointer reads ([4a54750](https://github.com/R0b-1ns/cairn/commit/4a54750c4074be7e60bbcc72678e631185020fc3))
* **init:** accept dotted repo names as children and tighten child init tests ([58a340d](https://github.com/R0b-1ns/cairn/commit/58a340dfb97033b7c0fe119a2a05771f14202b76))
* **locate:** attach a child only through a linked path and a matching parent id ([0550901](https://github.com/R0b-1ns/cairn/commit/05509014a1acbab1364660a09ac8580cc25dca03))
* **project:** read children without migrating them, fail closed on unlink, allow re-linking a local child ([56bc475](https://github.com/R0b-1ns/cairn/commit/56bc4751e5349b6ab5a4c673892c8cfd6ad90592))
* validate and sanitize link data read from untrusted files ([cf7f80a](https://github.com/R0b-1ns/cairn/commit/cf7f80ae663578e864cfe7df9f3d3e6efe094ace))

## [1.3.0](https://github.com/R0b-1ns/cairn/compare/v1.2.0...v1.3.0) (2026-11-01)


### Features

* adapt task tracking (numbering modes, ticket refs, author and branch, interactive init) ([2ec6468](https://github.com/R0b-1ns/cairn/commit/2ec6468685de7cf86ed7af6f08a70328440cadfb))
* **add:** record author and branch, assign task numbers, accept a ref ([595eb9c](https://github.com/R0b-1ns/cairn/commit/595eb9cc3892a3b9aed85f2827922db44fad56a2))
* **gitinfo:** read the branch and the git identity ([2d62b68](https://github.com/R0b-1ns/cairn/commit/2d62b6874faf27a27912d86f87a6f423eff1e250))
* **init:** ask the questions interactively in a terminal ([feb9b66](https://github.com/R0b-1ns/cairn/commit/feb9b66b2666f2c770505c7de82b5ee00cb64dc0))
* **init:** choose a task numbering mode ([f1a9ab3](https://github.com/R0b-1ns/cairn/commit/f1a9ab3bb683fbe8a28bdc4c5705a933a5867005))
* print a compact origin on brief note lines ([560bef1](https://github.com/R0b-1ns/cairn/commit/560bef1c9cd31fdf15451fbe407a03c82da5b7d8))
* **prompt:** ask one line at a time with a default and validation ([03e71f8](https://github.com/R0b-1ns/cairn/commit/03e71f80b43fb52fd7ff02a298d7873356a6e0da))
* show author, branch and refs; filter by them; round-trip them in export and import ([e9ca543](https://github.com/R0b-1ns/cairn/commit/e9ca54364cf44f1c984d95b8aa5833852e479595))
* show the task mode in context and the refs in brief and show JSON ([47d2294](https://github.com/R0b-1ns/cairn/commit/47d2294a71482167f06a714b92c43e5489aa696e))
* **store:** schema version 4 with the first migration ([1e9cfce](https://github.com/R0b-1ns/cairn/commit/1e9cfce53be4d780aff313891ca6a0a5d8bb1593))
* **store:** store author, branch and ref; number auto tasks ([101d069](https://github.com/R0b-1ns/cairn/commit/101d0695f2cde4390503097c930adc862abd3ca9))
* **tasks:** task modes, URL template and field validation ([2c50435](https://github.com/R0b-1ns/cairn/commit/2c50435cc14e10f226b2f00d06a926f3aa180de8))
* **view:** branch and author columns, filters and sorting ([bcc8bfa](https://github.com/R0b-1ns/cairn/commit/bcc8bfa5eb1baeca6b0e2ae75ad1a2f6dc35d25a))


### Bug Fixes

* bound a hung git to one timeout per command, not one per git call ([a317a8f](https://github.com/R0b-1ns/cairn/commit/a317a8f669c4a8be3d203f116a66c90dd5484042))
* cancel init on Ctrl-C, bound git calls, clearer version and env errors ([2f698f0](https://github.com/R0b-1ns/cairn/commit/2f698f06dc37fb6586ef52c7e67947b5e6185fb0))
* escape the computed ticket link and write the origin as Name &lt;email&gt; ([d7bfaa1](https://github.com/R0b-1ns/cairn/commit/d7bfaa1d34bc3fdc0f17329700e44f5fe5a29d37))
* **gitinfo:** read the branch of a repository with no commit yet ([d6998e2](https://github.com/R0b-1ns/cairn/commit/d6998e2d48599384861435bf218b82f45c9255f9))
* **init:** allow --tasks custom with its pattern ([86848f7](https://github.com/R0b-1ns/cairn/commit/86848f7414bc1d831a95fe78cb41c8b31e6c811b))
* reuse a canceled prompt read instead of starting a second reader ([bb415d6](https://github.com/R0b-1ns/cairn/commit/bb415d6616d45ee9bbd8bfc1a147a6e725f3ba62))
* **store:** keep the migration's lock while copying the backup ([16e61bc](https://github.com/R0b-1ns/cairn/commit/16e61bc147368573f0d387c64cd1ac3cc3a7bd09))
* **store:** refuse a task outside the pattern and exhausted numbering ([1a22e22](https://github.com/R0b-1ns/cairn/commit/1a22e224fb495acf20cb04ae4088ac0253084cc8))
* **store:** verify the old schema before migrating it ([23d4464](https://github.com/R0b-1ns/cairn/commit/23d44647f5b6445aeda6435d7b2acaf51cb8fd22))

## [1.2.0](https://github.com/R0b-1ns/cairn/compare/v1.1.0...v1.2.0) (2026-11-01)


### Features

* **init:** say on stderr when a newer release exists ([f896777](https://github.com/R0b-1ns/cairn/commit/f8967778217e3bddf652f8bec034a03e00133328))
* **update:** add cairn update and a new-version notice on init ([b2aadaf](https://github.com/R0b-1ns/cairn/commit/b2aadaf4b4cd21bf90727aa6b2c8f3f71f0eb203))
* **update:** add cairn update, which replaces the binary after checking its SHA-256 ([f1bd5e8](https://github.com/R0b-1ns/cairn/commit/f1bd5e8e05523a936d7ac211334a85d762c7c9a1))

## [1.1.0](https://github.com/R0b-1ns/cairn/compare/v1.0.1...v1.1.0) (2026-11-01)


### Features

* add cairn view, a browsable site of the notes ([4ea172d](https://github.com/R0b-1ns/cairn/commit/4ea172d9de76983589e45b0e59b0c3d62a297464))
* **install:** add an install script that verifies the release checksum ([361edec](https://github.com/R0b-1ns/cairn/commit/361edeced0c00dec923dd4d17af87f05b10f5971))
* **install:** add an install script that verifies the release checksum ([314f304](https://github.com/R0b-1ns/cairn/commit/314f304313de2a034380a5f16f3996ce78a765ee))
* **view:** add the cairn view command ([b53b4be](https://github.com/R0b-1ns/cairn/commit/b53b4be57da3200599cdec6d06130aac1ba614c4))
* **view:** collect notes and entries for the site ([96a655e](https://github.com/R0b-1ns/cairn/commit/96a655e4443e7668a71cf13cf61449cf03e5f43c))
* **view:** render the static site ([1222256](https://github.com/R0b-1ns/cairn/commit/12222568a79a4c72a1fbdebcb66f1d57bc2814f3))
* **view:** serve the generated site on localhost ([5e16d1c](https://github.com/R0b-1ns/cairn/commit/5e16d1ca9a0e73fccd8bc905fda1b3e3f75a9ddd))


### Bug Fixes

* **export:** sanitize each stored value where the Markdown prints it ([e1ae83b](https://github.com/R0b-1ns/cairn/commit/e1ae83b6c02f395a7691dcea147e5f64a463baa3))
* **view:** explain an entry view whose entries are all closed ([ce79b74](https://github.com/R0b-1ns/cairn/commit/ce79b742eb4efff74e88fc7238c2f51720fecc60))
* **view:** follow the task rule of the CLI and offer per-view filter values ([d23aa80](https://github.com/R0b-1ns/cairn/commit/d23aa8081795f5d7814d02dc60a47c79dbe86443))
* **view:** let a second Ctrl-C stop the server at once ([89d6b97](https://github.com/R0b-1ns/cairn/commit/89d6b97faf45595efdd07d5dbf2f12b8cedae2c1))
* **view:** make "Download as Markdown" download when opened from disk ([371dab8](https://github.com/R0b-1ns/cairn/commit/371dab8ade163971ffe1289bcf53441ca06ccb1a))
* **view:** mark closed entries in a downloaded entry view ([62c357f](https://github.com/R0b-1ns/cairn/commit/62c357f54092a4ca4f4f90f61100343cd05de4e9))
* **view:** refuse cross-site requests to the served site ([876f9a7](https://github.com/R0b-1ns/cairn/commit/876f9a7ac5232d7260d5d86dc5cc4256fdd4484b))
* **view:** render the site Markdown from stored values, then sanitize it ([e71a6d2](https://github.com/R0b-1ns/cairn/commit/e71a6d2c60fcc5eecca37877ff7d8dc037d10ecd))
* **view:** sanitize every stored text that reaches the site ([5c541b8](https://github.com/R0b-1ns/cairn/commit/5c541b879e27b5e7a7d55bce861d87be8ba387d5))
* **view:** title the entry-view download and say when it is empty ([7c5565f](https://github.com/R0b-1ns/cairn/commit/7c5565f8bf25e15811cdbfab3b32df0839753c8e))

## [1.0.1](https://github.com/R0b-1ns/cairn/compare/v1.0.0...v1.0.1) (2026-10-30)


### Bug Fixes

* clean exported files and refuse to truncate a swapped or hard-linked file ([f7908a5](https://github.com/R0b-1ns/cairn/commit/f7908a551d139ce1f9980ae1ed4c9e35aa3d5a3d))
* clean exported files and refuse to truncate swapped or hard-linked files ([a194ae4](https://github.com/R0b-1ns/cairn/commit/a194ae4ee0ac075f2cb78d29cb38c2354903b78f))

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
