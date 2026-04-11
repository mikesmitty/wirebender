# Changelog

## [0.2.0](https://github.com/mikesmitty/wirebender/compare/dxf2bend-v0.1.0...dxf2bend-v0.2.0) (2026-04-11)


### ⚠ BREAKING CHANGES

* Update module golang.org/x/image to v0.39.0 ([#9](https://github.com/mikesmitty/wirebender/issues/9))
* Update module golang.org/x/image to v0.38.0 ([#7](https://github.com/mikesmitty/wirebender/issues/7))

### Features

* **dxf2bend:** add visual previews and path selection ([fe6a0c0](https://github.com/mikesmitty/wirebender/commit/fe6a0c04d74c4f9365f1e21f498f704f50f4c3c2))
* switch to metro-rp2350 target and update Go toolchain ([2805e8c](https://github.com/mikesmitty/wirebender/commit/2805e8c4f191942f54b2f7bf52767d630428e176))
* Update module golang.org/x/image to v0.38.0 ([#7](https://github.com/mikesmitty/wirebender/issues/7)) ([7001b24](https://github.com/mikesmitty/wirebender/commit/7001b242b2e78df679d32c1db49daedb9cdd24ee))
* Update module golang.org/x/image to v0.39.0 ([#9](https://github.com/mikesmitty/wirebender/issues/9)) ([107209b](https://github.com/mikesmitty/wirebender/commit/107209b1608627ffdb3a27d969926455d8ac73e1))


### Bug Fixes

* change yaml output indentation to 2 spaces from 4 ([21b6ca5](https://github.com/mikesmitty/wirebender/commit/21b6ca5ac24100871daf1be6b099071cd9ef90b7))

## 0.1.0 (2026-03-08)


### Features

* add dxf2bend CLI tool for converting DXF paths to G-code ([a2fc413](https://github.com/mikesmitty/wirebender/commit/a2fc413ca37227843b9cf4ebba26c08a13eef8f7))
* add warning when multiple paths are found in DXF ([1ed39d5](https://github.com/mikesmitty/wirebender/commit/1ed39d5b6dfc8e6da99d6755044bea80b3e84d5f))
* **dxf2bend:** add arc, spline, multipath, simplify, and reverse support ([c651f83](https://github.com/mikesmitty/wirebender/commit/c651f833a6aa540dabeae467d2902c3210035172))
* **dxf2bend:** add cobra CLI, material library, and config file support ([e95a0a0](https://github.com/mikesmitty/wirebender/commit/e95a0a0ec83d35efc1295fd5df155f9bc857d73b))
* **dxf2bend:** add speed optimization, strict mode, and output summary ([3f9f075](https://github.com/mikesmitty/wirebender/commit/3f9f0753dada4c9d6a1ff7f65a31befc1163e388))
* rename FEED axis to L (Linear), convert to mm, add M200 roller config ([70afe92](https://github.com/mikesmitty/wirebender/commit/70afe92489ccde4248f6b41f88ce88293fc8a934))
