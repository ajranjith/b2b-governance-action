# Publishing to GitHub Marketplace

## Prerequisites

1. Create the repo `ajranjith/b2b-governance-action` on GitHub
2. Make it **public**
3. Push these files to `main` branch

## Steps

### 1. Initialize repo and push

```bash
cd /path/to/b2b-governance-action

git init
git add .
git commit -m "feat: Initial GitHub Action for GRES B2B Governance"
git branch -M main
git remote add origin https://github.com/ajranjith/b2b-governance-action.git
git push -u origin main
```

### 2. Create version tag

```bash
# Create versioned tag
git tag v4.0.0
git push origin v4.0.0

# Create/update moving major version tag (users reference @v4)
git tag -f v4
git push -f origin v4
```

### 3. Publish to Marketplace

1. Go to https://github.com/ajranjith/b2b-governance-action
2. Click **Releases** → **Draft a new release**
3. Select tag `v4.0.0`
4. Check **"Publish this Action to the GitHub Marketplace"**
5. Fill in:
   - **Primary category**: Code quality
   - **Secondary category**: Continuous integration
6. Add release notes:
   ```
   ## GRES B2B Governance v4.0.0

   Initial release of the GitHub Action wrapper.

   ### Features
   - Cross-platform binary download (linux/darwin/windows × amd64/arm64)
   - SHA256 checksum verification
   - Support for --verify, --watch, --shadow modes
   - Cap-aware threshold gating with JUnit/SARIF output
   ```
7. Click **Publish release**

## Updating the Action

When releasing new versions:

```bash
# Create new version tag
git tag v4.0.1
git push origin v4.0.1

# Move the major version tag (so @v4 users get updates)
git tag -f v4
git push -f origin v4
```

## File Structure

```
.
├── action.yml              # Action definition (Marketplace reads this)
├── README.md               # Shown on Marketplace listing
├── LICENSE                 # Required for Marketplace
├── PUBLISH.md              # This file (not shown on Marketplace)
└── scripts/
    └── install-and-run.sh  # Installer script
```
