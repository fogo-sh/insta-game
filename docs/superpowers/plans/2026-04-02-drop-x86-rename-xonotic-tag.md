# Drop x86_64 Services & Rename Xonotic Image Tag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all x86_64 game server infrastructure and consolidate to ARM64-only, while fixing image tag naming so Xonotic uses `:xonotic` and QSS-M uses `:qssm`.

**Architecture:** Three independent areas of change — CI workflows, Docker directory cleanup, and Pulumi config — each committed separately. No logic changes to Lambda or sidecar code; only wiring and resource definitions change.

**Tech Stack:** GitHub Actions, Docker/GHCR, Pulumi (Python), AWS ECS/Lambda

---

### Task 1: Delete x86_64 CI workflows

**Files:**
- Delete: `.github/workflows/publish-xonotic.yml`
- Delete: `.github/workflows/publish-qssm.yml`

- [ ] **Step 1: Delete the two x86_64 workflow files**

```bash
rm .github/workflows/publish-xonotic.yml
rm .github/workflows/publish-qssm.yml
```

- [ ] **Step 2: Verify only ARM workflows remain**

```bash
ls .github/workflows/
```

Expected output:
```
publish-qssm-arm.yml
publish-xonotic-arm.yml
```

- [ ] **Step 3: Commit**

```bash
git add -A .github/workflows/publish-xonotic.yml .github/workflows/publish-qssm.yml
git commit -m "ci: remove x86_64 Xonotic and QSS-M build workflows"
```

---

### Task 2: Update and rename ARM CI workflows

**Files:**
- Delete + recreate: `.github/workflows/publish-xonotic-arm.yml` → `.github/workflows/publish-xonotic.yml`
- Delete + recreate: `.github/workflows/publish-qssm-arm.yml` → `.github/workflows/publish-qssm.yml`

- [ ] **Step 1: Replace publish-xonotic-arm.yml with publish-xonotic.yml**

Delete the old file and create the new one at the new path:

```bash
rm .github/workflows/publish-xonotic-arm.yml
```

Write `.github/workflows/publish-xonotic.yml` with the following content:

```yaml
name: Create and publish the Xonotic Docker image

on:
  push:
    branches: ["main"]
    paths:
      - "docker-containers/xonotic-arm/**"

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push-image:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@v2

      - name: Download Xonotic and Setup Build Dependencies
        run: |
          cd ./docker-containers/xonotic-arm
          make download && make clean

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to the Container registry
        uses: docker/login-action@v1.12.0
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push Docker image
        uses: docker/build-push-action@v2.9.0
        with:
          context: ./docker-containers/xonotic-arm/
          platforms: linux/arm64
          push: true
          tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:xonotic
```

- [ ] **Step 2: Replace publish-qssm-arm.yml with publish-qssm.yml**

```bash
rm .github/workflows/publish-qssm-arm.yml
```

Write `.github/workflows/publish-qssm.yml` with the following content:

```yaml
name: Create and publish the QSS-M Docker image

on:
  push:
    branches: ["main"]
    paths:
      - "docker-containers/qssm/**"

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push-image:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@v2

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to the Container registry
        uses: docker/login-action@v1.12.0
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push Docker image
        uses: docker/build-push-action@v2.9.0
        with:
          context: ./docker-containers/qssm/
          platforms: linux/arm64
          push: true
          tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:qssm
```

- [ ] **Step 3: Verify workflow files**

```bash
ls .github/workflows/
```

Expected:
```
publish-qssm.yml
publish-xonotic.yml
```

- [ ] **Step 4: Commit**

```bash
git add -A .github/workflows/
git commit -m "ci: rename ARM workflows, use :xonotic/:qssm tags, add path filters"
```

---

### Task 3: Delete the x86_64 Xonotic Docker directory

**Files:**
- Delete: `docker-containers/xonotic/` (entire directory)

- [ ] **Step 1: Delete the directory**

```bash
rm -rf docker-containers/xonotic/
```

- [ ] **Step 2: Verify only the ARM and qssm directories remain**

```bash
ls docker-containers/
```

Expected:
```
qssm/
xonotic-arm/
```

- [ ] **Step 3: Commit**

```bash
git add -A docker-containers/xonotic/
git commit -m "chore: remove x86_64 Xonotic Docker directory"
```

---

### Task 4: Update Pulumi to remove x86_64 services and fix image tags

**Files:**
- Modify: `pulumi/__main__.py`

- [ ] **Step 1: Remove the x86_64 `xonotic` and `qssm` GameService resources, rename ARM variables, fix image tags, and update the Lambda GAMES map**

Replace the entire ECS + Lambda section of `pulumi/__main__.py`. The file currently has 365 lines. Replace from line 222 (the `xonotic = GameService(...)` block) to the end of the file with the following:

```python
xonotic = GameService(
    "xonotic-arm",
    game_name="xonotic-arm",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:xonotic",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=xonotic_data_url,
)

qssm = GameService(
    "qssm-arm",
    game_name="qssm-arm",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:qssm",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=qss_m_data_url,
)

# ---- Lambda ----

launcher = aws.lambda_.Function(
    "launcher",
    name=regional_name("launcher"),
    runtime="python3.12",
    handler="launcher.handler",
    timeout=120,
    role=lambda_role.arn,
    code=pulumi.AssetArchive(
        {
            "launcher.py": pulumi.FileAsset("../lambda/launcher/launcher.py"),
        }
    ),
    environment=aws.lambda_.FunctionEnvironmentArgs(
        variables=pulumi.Output.all(
            sidecar_token,
            xonotic.service_name,
            cluster.name,
            qssm.service_name,
        ).apply(
            lambda args: {
                "SIDECAR_TOKEN": args[0],
                "ECS_CLUSTER": args[2],
                "GAMES": json.dumps(
                    {
                        "xonotic": {
                            "service_name": args[1],
                            "sidecar_port": 5001,
                        },
                        "qssm": {
                            "service_name": args[3],
                            "sidecar_port": 5001,
                        },
                    }
                ),
            }
        ),
    ),
)

launcher_url = aws.lambda_.FunctionUrl(
    "launcher-url",
    function_name=launcher.name,
    authorization_type="NONE",
)

aws.lambda_.Permission(
    "launcher-url-permission",
    action="lambda:InvokeFunctionUrl",
    function=launcher.name,
    principal="*",
    function_url_auth_type="NONE",
)

aws.lambda_.Permission(
    "launcher-function-url-invoke-permission",
    action="lambda:InvokeFunction",
    function=launcher.name,
    principal="*",
    invoked_via_function_url=True,
)

pulumi.export("prod_url", launcher_url.function_url)
```

Note: the Pulumi logical resource names (`"xonotic-arm"`, `"qssm-arm"`) are kept stable intentionally — changing them would cause Pulumi to destroy and recreate the ECS resources. The Python variable names (`xonotic`, `qssm`) are just local references.

- [ ] **Step 2: Run ruff to verify no lint errors**

```bash
cd pulumi
uv run ruff check .
uv run ruff format --check .
```

Expected: no errors or formatting issues.

- [ ] **Step 3: Run pulumi preview to verify the plan**

```bash
cd pulumi
uv run pulumi preview
```

Expected: the preview should show:
- 2 ECS services being **deleted** (the x86_64 `xonotic` and `qssm` services and their task definitions / log groups)
- 2 ECS services being **updated** (image tag changes: `:xonotic-arm` → `:xonotic` and `:qssm-arm` → `:qssm`)
- Lambda function environment being **updated** (GAMES map reduced from 4 to 2 entries)
- No unexpected replacements or deletions

- [ ] **Step 4: Commit**

```bash
git add pulumi/__main__.py
git commit -m "infra: remove x86_64 ECS services, rename image tags to :xonotic/:qssm"
```

---

### Task 5: Apply Pulumi changes

- [ ] **Step 1: Run pulumi up**

```bash
cd pulumi
uv run pulumi up
```

Confirm the preview matches expectations (same as Task 4 Step 3), then select `yes`.

- [ ] **Step 2: Verify the stack outputs**

```bash
uv run pulumi stack output prod_url
```

Expected: the Lambda Function URL is still present and unchanged.

- [ ] **Step 3: Smoke-test the launcher**

```bash
curl "<prod_url>?game=xonotic&operation=status"
curl "<prod_url>?game=qssm&operation=status"
```

Expected: both return a JSON response with `"status": "offline"` (or `"starting"` if tasks are still draining). The old `?game=xonotic-arm` and `?game=qssm-arm` keys should now return an error or unknown-game response from the Lambda.

- [ ] **Step 4: Commit (no code changes — this step is infrastructure-only)**

No commit needed for this step; `pulumi up` updates remote state only.
