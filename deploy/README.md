# Deploying waymeet-backend to a DigitalOcean Droplet

A single 2 GB / 1 vCPU Droplet (~$12/mo) runs the whole stack with Docker
Compose: **api**, **worker**, **Postgres+PostGIS**, **Redis**, **MinIO**, and
**Caddy** (auto HTTPS). Good for a dev/staging environment.

```
Internet ──▶ Caddy :443 ──┬─▶ api:8080      (Go HTTP API)
  (TLS)                    └─▶ minio:9000    (media, presigned URLs)
                              worker          (asynq jobs, no public port)
                              postgres ◀── api/worker/migrate
                              redis    ◀── api/worker
```

## 1. Create the server

1. DigitalOcean Console → **Create → Droplets**.
2. Region: pick one near you (e.g. **SGP1 Singapore**, **NYC**, **FRA1**).
3. Image: **Ubuntu 24.04 (LTS)**.
4. Size: **Basic → Regular → 2 GB / 1 vCPU / 50 GB ($12/mo)**. (The $6/1 GB
   plan is too small for Postgres+Redis+MinIO+Go builds.)
5. Authentication: add your **SSH key**.
6. **Advanced → Add Initialization scripts (user data)**: paste
   `deploy/cloud-init.yaml` (edit your SSH key in it first; it adds swap,
   installs Docker, and configures the firewall).
7. Create. Note the **public IPv4** (e.g. `203.0.113.45`).

> IPv4 and a generous bandwidth allowance are included in the Droplet price —
> no separate address charge.

No domain needed: we use `sslip.io`, where `anything.203-0-113-45.sslip.io`
resolves to `203.0.113.45`. (Have a domain? Point A records at the IP and use
real hostnames in `.env.prod` instead — nothing else changes.)

## 2. Ship the code to the server

From your Windows machine, in `waymeet-backend/`:

```powershell
# Sync the repo (needs scp/rsync via Git Bash, or use the Bash tool's rsync).
# Simplest: tar it up and copy, or use `git clone` on the server if you push it.
scp -r . deploy@203.0.113.45:~/waymeet-backend
```

> Tip: the backend isn't a git repo yet. Either `git init && push` to a remote
> and `git clone` on the server, or copy the folder as above. Exclude
> `*.exe`, `*.log`, and `build/` to keep the upload small.

## 3. Configure secrets (on the server)

```bash
ssh deploy@203.0.113.45
cd ~/waymeet-backend

cp deploy/.env.prod.example deploy/.env.prod
nano deploy/.env.prod        # set the IP (203-0-113-45), passwords, JWT_SECRET

# generate strong values
openssl rand -base64 48      # -> JWT_SECRET
openssl rand -base64 24      # -> POSTGRES_PASSWORD / S3_SECRET_KEY

# Firebase Cloud Messaging service account (for push):
mkdir -p deploy/secrets
# copy your waymeet-be42f-firebase-adminsdk-*.json here as fcm.json
mv waymeet-be42f-firebase-adminsdk-*.json deploy/secrets/fcm.json
```

Make sure these match in `.env.prod`:
`POSTGRES_PASSWORD` == the password inside `DATABASE_URL`, and
`API_DOMAIN` / `MEDIA_DOMAIN` / `S3_ENDPOINT` all use the same IP.

## 4. Launch

Run from the `waymeet-backend/` directory (build context = repo root):

```bash
docker compose -f deploy/docker-compose.prod.yml --project-directory . --env-file deploy/.env.prod up -d --build
```

This builds the api/worker images, starts the datastores, runs the
`minio-init` (creates the bucket) and `migrate` (applies DB migrations)
one-shots, then starts Caddy which fetches TLS certs.

## 5. Verify

```bash
docker compose -f deploy/docker-compose.prod.yml --project-directory . ps          # all Up / healthy
docker compose -f deploy/docker-compose.prod.yml --project-directory . logs -f api # watch startup

curl https://api.203-0-113-45.sslip.io/healthz               # -> ok
```

Point the Flutter app's API base URL at `https://api.<ip>.sslip.io`.

## Day-2 commands

```bash
# Update after new code is synced:
docker compose -f deploy/docker-compose.prod.yml --project-directory . --env-file deploy/.env.prod up -d --build api worker

# Re-run migrations:
docker compose -f deploy/docker-compose.prod.yml --project-directory . --env-file deploy/.env.prod up migrate

# Tail logs / restart / stop:
docker compose -f deploy/docker-compose.prod.yml --project-directory . logs -f
docker compose -f deploy/docker-compose.prod.yml --project-directory . restart api
docker compose -f deploy/docker-compose.prod.yml --project-directory . down        # add -v to wipe data
```

## Backups (cheap insurance)

```bash
# Postgres dump to home dir (cron it daily):
docker compose -f deploy/docker-compose.prod.yml --project-directory . exec -T postgres \
  pg_dump -U waymeet waymeet | gzip > ~/backup-$(date +%F).sql.gz
```

DigitalOcean Droplet snapshots/backups are also available in the console
(weekly backups add ~20% of the Droplet cost).

## Cost watch

- 2 GB Droplet: ~$12/mo (IPv4 + bandwidth included)
- Optional automatic backups: ~$2.40/mo
- Slightly above the $5–10 target; the 1 GB ($6) plan can't run the full stack.
  To hit ~$6: offload media to **Cloudflare R2** (free, S3-compatible), drop the
  `minio`/`minio-init` services, point `S3_*` at R2, and use a 1 GB Droplet.
