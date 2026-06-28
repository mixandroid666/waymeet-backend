# CI/CD: push to main → auto-deploy to the Droplet

## How it works

```
git push origin main
        │
        ▼
GitHub Actions (.github/workflows/deploy.yml)
        │  SSH into the Droplet
        ▼
Droplet:  git reset --hard origin/main
          docker compose up -d --build   # rebuilds api/worker, runs migrate one-shot
```

The Go build runs **on the Droplet** (2 GB RAM + 2 GB swap). The `migrate`
one-shot in `docker-compose.prod.yml` applies any new DB migrations on every
deploy. Secrets (`deploy/.env.prod`, `deploy/secrets/`) live **only on the
server** and are gitignored — they are never pulled or overwritten by a deploy.

## One-time setup

### 1. Create a CI-only SSH keypair
On your machine (don't reuse your personal key):
```bash
ssh-keygen -t ed25519 -f ~/.ssh/waymeet_ci -C "github-actions-deploy" -N ""
```
Authorize its public half on the Droplet:
```bash
ssh deploy@159.223.46.94 "echo '$(cat ~/.ssh/waymeet_ci.pub)' >> ~/.ssh/authorized_keys"
```

### 2. Add GitHub repo secrets
Repo → **Settings → Secrets and variables → Actions → New repository secret**:

| Secret | Value |
|--------|-------|
| `DROPLET_HOST` | `159.223.46.94` |
| `DROPLET_USER` | `deploy` |
| `DROPLET_SSH_KEY` | contents of `~/.ssh/waymeet_ci` (the **private** key) |

### 3. Let the Droplet pull from GitHub (deploy key)
The Droplet needs read access to the repo. Add a **read-only deploy key**:
```bash
# on the Droplet:
ssh deploy@159.223.46.94
ssh-keygen -t ed25519 -f ~/.ssh/github_deploy -N ""
cat ~/.ssh/github_deploy.pub          # copy this
# add a host alias so git uses this key for github
cat >> ~/.ssh/config <<'EOF'
Host github.com
  IdentityFile ~/.ssh/github_deploy
  IdentitiesOnly yes
EOF
```
GitHub repo → **Settings → Deploy keys → Add deploy key** → paste the public key
(read-only is fine).

### 4. Convert the Droplet to a git checkout (one time)
The server currently holds a tarball copy. Replace it with a real clone while
**preserving the live env file and secrets**:
```bash
# on the Droplet, in ~:
mv waymeet-backend waymeet-backend.bak
git clone git@github.com:mixandroid666/waymeet-backend.git
cp waymeet-backend.bak/deploy/.env.prod      waymeet-backend/deploy/.env.prod
cp -r waymeet-backend.bak/deploy/secrets/.   waymeet-backend/deploy/secrets/ 2>/dev/null || true
# sanity check, then remove the backup once a deploy succeeds:
ls waymeet-backend/deploy/.env.prod waymeet-backend/deploy/secrets/
```

## Daily workflow after setup
```bash
git checkout -b feature/x      # work on a branch
# ... develop & test locally (make up; make migrate-up; make run) ...
git commit -m "..."
git push origin feature/x      # open a PR
# merge to main  ->  GitHub Actions deploys automatically
```
Watch a deploy under the repo's **Actions** tab. You can also trigger one
manually there via **Run workflow** (the `workflow_dispatch` trigger).
