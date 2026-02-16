-- Sync customer.name to contact (when contact present).
-- Rules:
-- 1) If contact is non-empty -> set name = contact
-- 2) If raw_name is empty -> preserve old name into raw_name
-- 3) Skip rows where the target name would violate UNIQUE(name)
--
-- Run:
--   docker exec -i erp_postgres psql -U nocodb -d nocodb -f /dev/stdin < scripts/customer_name_sync_contact.sql

\echo '--- preview: candidates (contact present, name different) ---'
SELECT COUNT(*) AS candidates
FROM p2rms15pepb5ciz.customers c
WHERE COALESCE(btrim(c.contact),'') <> ''
  AND btrim(c.contact) <> c.name;

\echo '--- preview: would-conflict (target name already exists on another customer) ---'
SELECT COUNT(*) AS would_conflict
FROM p2rms15pepb5ciz.customers c
WHERE COALESCE(btrim(c.contact),'') <> ''
  AND btrim(c.contact) <> c.name
  AND EXISTS (
    SELECT 1 FROM p2rms15pepb5ciz.customers c2
    WHERE c2.name = btrim(c.contact) AND c2.id <> c.id
  );

\echo '--- sample (up to 20) ---'
SELECT c.id, left(c.name, 40) AS old_name, left(btrim(c.contact), 40) AS new_name
FROM p2rms15pepb5ciz.customers c
WHERE COALESCE(btrim(c.contact),'') <> ''
  AND btrim(c.contact) <> c.name
  AND NOT EXISTS (
    SELECT 1 FROM p2rms15pepb5ciz.customers c2
    WHERE c2.name = btrim(c.contact) AND c2.id <> c.id
  )
ORDER BY c.id
LIMIT 20;

\echo '--- apply update (skipping conflicts) ---'
BEGIN;

WITH cand AS (
  SELECT c.id, c.name AS old_name, btrim(c.contact) AS new_name
  FROM p2rms15pepb5ciz.customers c
  WHERE COALESCE(btrim(c.contact),'') <> ''
    AND btrim(c.contact) <> c.name
), ok AS (
  -- keep only unique new_name within this batch, and also not conflicting with existing names
  SELECT c.id, c.old_name, c.new_name
  FROM cand c
  JOIN (
    SELECT new_name
    FROM cand
    GROUP BY new_name
    HAVING COUNT(*) = 1
  ) u ON u.new_name = c.new_name
  WHERE NOT EXISTS (
    SELECT 1 FROM p2rms15pepb5ciz.customers c2
    WHERE c2.name = c.new_name AND c2.id <> c.id
  )
)
UPDATE p2rms15pepb5ciz.customers c
SET
  raw_name = COALESCE(NULLIF(btrim(c.raw_name),''), ok.old_name),
  name = ok.new_name,
  updated_at = now()
FROM ok
WHERE c.id = ok.id;

COMMIT;

\echo '--- done ---'
