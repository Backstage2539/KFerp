# Roastery ERP (OpenClaw project)

This folder contains the deployment skeleton for Van's roastery system.

## Reverse proxy / HTTPS (Traefik)

- Domain: `erp.stopcloud.site`
- External port: `8765` (HTTPS)
- ACME: Let's Encrypt via DNS-01 (DNSPod)

### Required DNSPod credentials

Traefik's DNS-01 provider for DNSPod typically needs **both** an API ID and an API Token/Key (DNSPod calls it `login_token` formatted as `ID,Token`).

Set env vars (see `deploy/.env.example`):

- `DNSPOD_API_ID`
- `DNSPOD_API_KEY`

## Next steps

1. Copy `deploy/.env.example` to `deploy/.env` and fill credentials.
2. Deploy via DSM Container Manager using `deploy/docker-compose.yml`.
3. Port-forward TCP 8765 on your router to the NAS host running Traefik.

