# Device definitions

Device definitions describe hardware and image-compatibility facts. They are
separate from deployment manifests in `examples/`: a deployment manifest says
where to obtain an image, while a device definition says what may safely be
built for a physical product.

Each product lives at `devices/<vendor>/<model>-<revision>/` and is split into
small files so that upstream compatibility, hardware facts, and local deltas
are independently reviewable:

- `device.yaml` — product identity and inheritance
- `hardware.yaml` — SoC, storage, NVMEM, Ethernet, and radio topology
- `openwrt.yaml` — upstream target and image-format compatibility
- `overlays/*.yaml` — product-specific differences from the declared base

Values marked `verified: false` are hypotheses inherited from the base device.
They must not become inputs to generated DTS or image metadata until an AX23V
device dump or direct hardware test verifies them. Calibration and complete MTD
dumps are device-specific secrets and must never be committed.
