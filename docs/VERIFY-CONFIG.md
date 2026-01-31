# Verify Gating Configuration

The `gres-b2b verify` command reads `.b2b/config.yml` from the workspace root to determine pass/fail thresholds.

## Configuration File

Place a `.b2b/config.yml` in your project root:

```yaml
fail_on_red: true
allow_amber: false
max_red: 0
max_amber: 5
```

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fail_on_red` | bool | `true` | Any RED violation fails the gate |
| `allow_amber` | bool | `false` | Whether AMBER violations are tolerated |
| `max_red` | int (optional) | unset | Maximum RED violations before failing |
| `max_amber` | int (optional) | unset | Maximum AMBER violations before failing |

All fields are optional. Omitted fields use the defaults shown above.

## Gating Rules

Rules are evaluated in this order:

1. **Numeric caps** (if set):
   - If `max_red` is set and `redCount > max_red` -> **FAIL**
   - If `max_amber` is set and `amberCount > max_amber` -> **FAIL**

2. **Boolean rules**:
   - If `fail_on_red` is `true` and `redCount > 0` -> **FAIL**
   - If `allow_amber` is `false` and `amberCount > 0` -> **FAIL**

3. If no rule triggers a failure -> **PASS**

Precedence: caps always win. If a cap is exceeded, the gate fails regardless of `fail_on_red` or `allow_amber`.

## Priority: Caps vs Booleans

Caps do **not** override `fail_on_red`. If `fail_on_red: true`, any RED violation fails regardless of `max_red`.

To use `max_red` for tolerable debt, set `fail_on_red: false`:

```yaml
# Allow up to 5 reds (tolerable debt)
fail_on_red: false
max_red: 5
allow_amber: true
max_amber: 10
```

If `fail_on_red: true` is set alongside `max_red`, both are checked and either can cause failure.

## Outputs

The `verify` command produces:

| File | Description |
|------|-------------|
| `.b2b/certificate.json` | Gating verdict with counts, caps, and pass/fail status |
| `.b2b/results.sarif` | SARIF 2.1.0 format for GitHub Security tab |
| `.b2b/junit.xml` | JUnit XML for CI/CD test reporting |
| stdout (HUD) | Human-readable box summary |

## Examples

### Strict: no violations allowed (default)

```yaml
fail_on_red: true
allow_amber: false
```

### Tolerable debt: allow some reds and ambers

```yaml
fail_on_red: false
allow_amber: true
max_red: 3
max_amber: 10
```

### Allow ambers but cap them

```yaml
fail_on_red: true
allow_amber: true
max_amber: 5
```

## Usage

```bash
# Run verify against scan results
gres-b2b verify

# With explicit config override (optional)
gres-b2b --config my-config.json verify

# Verify certificate signature
gres-b2b --verify-cert .b2b/certificate.json
```

The command reads `.b2b/results.json` (produced by `gres-b2b scan`) and applies gating rules from `.b2b/config.yml`. Exit code is `0` for PASS, `1` for FAIL.

