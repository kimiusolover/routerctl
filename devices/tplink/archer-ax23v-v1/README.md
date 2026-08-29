# TP-Link Archer AX23V v1 hardware profile

This profile is intentionally an overlay on `tplink-archer-ax23-v1`, not an
alias for it. The AX23V-specific Ethernet mapping is:

| Logical connector | Linux topology |
| --- | --- |
| WAN | GMAC1 / PHY0 |
| LAN1 | DSA port 1 |
| LAN2 | DSA port 2 |
| LAN3 | DSA port 3 |
| LAN4 | DSA port 4 |

The base AX23 v1 layout maps WAN to PHY4 and LAN1–LAN4 to ports 0–3. Applying
that DTS unchanged to AX23V causes WAN and LAN4 to be exchanged. Image
generation must consume `overlays/ax23v.yaml` to produce a small, reviewable
patch; it must never edit an upstream DTS in place.

`specialId: 4A500000` is a TP-Link safeloader support-list value for a factory
image. It is separate from `examples/ax23v/regulatory/JP/` and is not evidence
of an RF policy by itself.

Before promoting inherited values to verified AX23V facts, collect and retain
locally (not in Git) the MTD table, interface MAC addresses, and radio PHY
topology. In particular, verify the GPIO/button wiring, per-interface MAC
increments, and calibration partition contents.
