# TOOLS.md - Local Notes

Skills define _how_ tools work. This file is for _your_ specifics — the stuff that's unique to your setup.

## What Goes Here

Things like:

- Camera names and locations
- SSH hosts and aliases
- Preferred voices for TTS
- Speaker/room names
- Device nicknames
- Anything environment-specific

## Examples

```markdown
### Cameras

- living-room → Main area, 180° wide angle
- front-door → Entrance, motion-triggered

### SSH

- orderapp-prod → 1.12.242.58, port 22, user: root, key: /Users/yiiiple-work/.openclaw/workspace/openclaw_jj_ed25519
  - containers: erp_orderapp (app), erp_caddy (reverse proxy/443), erp_postgres (db)
  - postgres: user=nocodb db=nocodb schema=p2rms15pepb5ciz
  - url: https://erp.qacoohee.com/
  - auth: BasicAuth (APP_USER in erp_orderapp env; password also in env)
- (legacy) home-server → 192.168.1.100, user: admin

### TTS

- Preferred voice: "Nova" (warm, slightly British)
- Default speaker: Kitchen HomePod
```

## Why Separate?

Skills are shared. Your setup is yours. Keeping them apart means you can update skills without losing your notes, and share skills without leaking your infrastructure.

---

Add whatever helps you do your job. This is your cheat sheet.
