# Design: Drop x86_64 Services, Rename Xonotic Image Tag

**Date:** 2026-04-02

## Goal

Remove all x86_64 game server infrastructure (CI, Docker directory, Pulumi resources) and consolidate to ARM64-only. Fix tag naming inconsistency by renaming the Xonotic ARM image from `:xonotic-arm` to `:xonotic` and the QSS-M ARM image from `:qssm-arm` to `:qssm`. Add path-based CI triggers so workflows only run when their relevant Docker context changes.

## Changes

### CI (`.github/workflows/`)

| File | Action |
|---|---|
| `publish-xonotic.yml` | **Delete** (x86_64 Xonotic, publishes `:main`) |
| `publish-qssm.yml` | **Delete** (x86_64 QSS-M, publishes `:qssm`) |
| `publish-xonotic-arm.yml` | **Rename** to `publish-xonotic.yml`; change pushed tag from `:xonotic-arm` to `:xonotic`; add path filter on `docker-containers/xonotic-arm/**` |
| `publish-qssm-arm.yml` | **Rename** to `publish-qssm.yml`; change pushed tag from `:qssm-arm` to `:qssm`; add path filter on `docker-containers/qssm/**` |

All workflows currently trigger on every push to `main`. After this change each only triggers when its Docker context directory has changed.

### Docker directories

| Directory | Action |
|---|---|
| `docker-containers/xonotic/` | **Delete entirely** (x86_64 Xonotic with pre-built binary) |
| `docker-containers/xonotic-arm/` | Keep as-is (the sole Xonotic build going forward) |
| `docker-containers/qssm/` | Keep as-is (used for both architectures via QEMU) |

### Pulumi (`pulumi/__main__.py`)

- Remove the `xonotic` GameService resource (x86_64, image `:main`)
- Remove the `qssm` GameService resource (x86_64, image `:qssm`)
- Update `xonotic_arm` image from `ghcr.io/fogo-sh/insta-game:xonotic-arm` to `ghcr.io/fogo-sh/insta-game:xonotic`
- Update `qssm_arm` image from `ghcr.io/fogo-sh/insta-game:qssm-arm` to `ghcr.io/fogo-sh/insta-game:qssm`
- Rename the Python variables `xonotic_arm` → `xonotic` and `qssm_arm` → `qssm` for clarity
- Simplify the Lambda `GAMES` env var construction: drop from 6 `Output.all` args to 4, update key names

### Lambda env var (`GAMES` map)

Goes from 4 entries to 2. The `-arm` suffix is dropped from the game keys since there is now only one variant per game:

```json
{
  "xonotic": { "service_name": "<xonotic-arm service>", "sidecar_port": 5001 },
  "qssm":    { "service_name": "<qssm-arm service>",    "sidecar_port": 5001 }
}
```

API callers use `?game=xonotic` and `?game=qssm` (previously `?game=xonotic-arm` / `?game=qssm-arm`).

## Out of Scope

- No changes to sidecar service code
- No changes to Lambda handler logic
- No changes to VPC, IAM, or ECS cluster resources
- No renaming of `docker-containers/xonotic-arm/` directory (it still builds correctly as-is)
