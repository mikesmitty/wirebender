# Changelog

## 0.1.0 (2026-03-08)


### Features

* add degree-based coordinate system and expanded G-code support ([00ba272](https://github.com/mikesmitty/wirebender/commit/00ba272a9cbc55a410097d43763c51cd145e5a08))
* add dxf2bend CLI tool for converting DXF paths to G-code ([a2fc413](https://github.com/mikesmitty/wirebender/commit/a2fc413ca37227843b9cf4ebba26c08a13eef8f7))
* add warning when multiple paths are found in DXF ([1ed39d5](https://github.com/mikesmitty/wirebender/commit/1ed39d5b6dfc8e6da99d6755044bea80b3e84d5f))
* **dxf2bend:** add arc, spline, multipath, simplify, and reverse support ([c651f83](https://github.com/mikesmitty/wirebender/commit/c651f833a6aa540dabeae467d2902c3210035172))
* **dxf2bend:** add cobra CLI, material library, and config file support ([e95a0a0](https://github.com/mikesmitty/wirebender/commit/e95a0a0ec83d35efc1295fd5df155f9bc857d73b))
* **dxf2bend:** add speed optimization, strict mode, and output summary ([3f9f075](https://github.com/mikesmitty/wirebender/commit/3f9f0753dada4c9d6a1ff7f65a31befc1163e388))
* **firmware:** add G2/G3 arc support ([b94f0a2](https://github.com/mikesmitty/wirebender/commit/b94f0a268656d725aaac208a2e3c84e9d076a9af))
* **firmware:** add M401 wait-for-idle command ([ce59d0b](https://github.com/mikesmitty/wirebender/commit/ce59d0b0f92a920193d97a0465b15ca82e6aca95))
* **firmware:** add M500/M501 calibration save/restore ([3c32946](https://github.com/mikesmitty/wirebender/commit/3c32946173b8294fb7595a82bfb3c776006f4763))
* **firmware:** add motion command error feedback ([163497a](https://github.com/mikesmitty/wirebender/commit/163497abd79ca06c419386e3305861f8cc16f332))
* **firmware:** add parse error reporting and G-code comment support ([517e3c3](https://github.com/mikesmitty/wirebender/commit/517e3c3bce980eeee947a523b874b5eb38134991))
* **firmware:** add position limits, speed limits, and M211 command ([f166881](https://github.com/mikesmitty/wirebender/commit/f166881ee12d93c4b70eee1061bddcce1ef278e3))
* **firmware:** add servo health monitoring ([e3823e3](https://github.com/mikesmitty/wirebender/commit/e3823e3b4d517eb565bec95900752e1837fd1307))
* **firmware:** add servo heartbeat and startup verification ([7fcee70](https://github.com/mikesmitty/wirebender/commit/7fcee70250f2f2502c7fb6ab9647867a2865fd81))
* improve servo comms reliability and add .gitignore ([bc89c49](https://github.com/mikesmitty/wirebender/commit/bc89c494e61544b144fb570bb988c1481ca7a83b))
* improve startup calibration, add rotate limit, clean up TODO ([fc1f61c](https://github.com/mikesmitty/wirebender/commit/fc1f61c96b46e6106eba4f95aeddcd5452ae0cb6))
* rename FEED axis to L (Linear), convert to mm, add M200 roller config ([70afe92](https://github.com/mikesmitty/wirebender/commit/70afe92489ccde4248f6b41f88ce88293fc8a934))


### Bug Fixes

* **ci:** fix build-firmware and vet CI failures ([38f1b9e](https://github.com/mikesmitty/wirebender/commit/38f1b9e1461d71a611cc922918262f973d562202))
