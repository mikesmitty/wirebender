# Changelog

## [0.2.0](https://github.com/mikesmitty/wirebender/compare/dxf2bend-v0.1.0...dxf2bend-v0.2.0) (2026-07-17)


### ⚠ BREAKING CHANGES

* Update module golang.org/x/image to v0.44.0 ([#21](https://github.com/mikesmitty/wirebender/issues/21))
* Update module golang.org/x/image to v0.43.0 ([#18](https://github.com/mikesmitty/wirebender/issues/18))
* Update module golang.org/x/image to v0.42.0 ([#17](https://github.com/mikesmitty/wirebender/issues/17))
* Update module golang.org/x/image to v0.41.0 ([#14](https://github.com/mikesmitty/wirebender/issues/14))
* Update module golang.org/x/image to v0.40.0 ([#12](https://github.com/mikesmitty/wirebender/issues/12))
* Update module golang.org/x/image to v0.39.0 ([#9](https://github.com/mikesmitty/wirebender/issues/9))
* Update module golang.org/x/image to v0.38.0 ([#7](https://github.com/mikesmitty/wirebender/issues/7))

### Features

* **dxf2bend:** add visual previews and path selection ([fe6a0c0](https://github.com/mikesmitty/wirebender/commit/fe6a0c04d74c4f9365f1e21f498f704f50f4c3c2))
* switch to metro-rp2350 target and update Go toolchain ([2805e8c](https://github.com/mikesmitty/wirebender/commit/2805e8c4f191942f54b2f7bf52767d630428e176))
* Update module golang.org/x/image to v0.38.0 ([#7](https://github.com/mikesmitty/wirebender/issues/7)) ([7001b24](https://github.com/mikesmitty/wirebender/commit/7001b242b2e78df679d32c1db49daedb9cdd24ee))
* Update module golang.org/x/image to v0.39.0 ([#9](https://github.com/mikesmitty/wirebender/issues/9)) ([107209b](https://github.com/mikesmitty/wirebender/commit/107209b1608627ffdb3a27d969926455d8ac73e1))
* Update module golang.org/x/image to v0.40.0 ([#12](https://github.com/mikesmitty/wirebender/issues/12)) ([f0db80e](https://github.com/mikesmitty/wirebender/commit/f0db80ed73085a70e54d1be11e048434414d2e02))
* Update module golang.org/x/image to v0.41.0 ([#14](https://github.com/mikesmitty/wirebender/issues/14)) ([5bf3ccd](https://github.com/mikesmitty/wirebender/commit/5bf3ccd79e9610fddc63c992da0215e9d2d09ac0))
* Update module golang.org/x/image to v0.42.0 ([#17](https://github.com/mikesmitty/wirebender/issues/17)) ([29d483b](https://github.com/mikesmitty/wirebender/commit/29d483b08487a803851e1c5b91882d9f636cb0e2))
* Update module golang.org/x/image to v0.43.0 ([#18](https://github.com/mikesmitty/wirebender/issues/18)) ([c2d30ad](https://github.com/mikesmitty/wirebender/commit/c2d30ad536819439f397f386e70458d9f00d4095))
* Update module golang.org/x/image to v0.44.0 ([#21](https://github.com/mikesmitty/wirebender/issues/21)) ([2c87687](https://github.com/mikesmitty/wirebender/commit/2c876874a15db24348c3e433d0f04ac614eabc6b))


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
