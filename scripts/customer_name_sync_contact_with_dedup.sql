-- Sync customer.name to contact (when contact present) with de-dup.
-- Rules:
-- 1) If contact present -> target base = btrim(contact)
-- 2) If base name is unique across all customers (except self) -> name = base
-- 3) If base conflicts -> name = base || '-' || id   (guaranteed unique)
-- 4) If raw_name is empty -> preserve old name into raw_name

\echo '--- preview: candidates (contact present, name different) ---'
SELECT COUNT(*) AS candidates
FROM p2rms15pepb5ciz.customers c
WHERE COALESCE(btrim(c.contact),'') <> ''
  AND btrim(c.contact) <> c.name;

\echo '--- preview: conflicts (base name already exists on another customer) ---'
SELECT COUNT(*) AS base_conflicts
FROM p2rms15pepb5ciz.customers c
WHERE COALESCE(btrim(c.contact),'') <> ''
  AND btrim(c.contact) <> c.name
  AND EXISTS (
    SELECT 1 FROM p2rms15pepb5ciz.customers c2
    WHERE c2.name = btrim(c.contact) AND c2.id <> c.id
  );

\echo '--- apply update (dedup with -id) ---'
BEGIN;

WITH cand AS (
  SELECT c.id,
         c.name AS old_name,
         btrim(c.contact) AS base
  FROM p2rms15pepb5ciz.customers c
  WHERE COALESCE(btrim(c.contact),'') <> ''
    AND btrim(c.contact) <> c.name
), ranked AS (
  SELECT c.*, row_number() OVER (PARTITION BY c.base ORDER BY c.id) AS rn
  FROM cand c
), resolved AS (
  SELECT
    r.id,
    r.old_name,
    CASE
      -- if multiple rows share same base in this batch: only rn=1 can try base
      WHEN r.rn = 1 AND NOT EXISTS (
        SELECT 1 FROM p2rms15pepb5ciz.customers c2
        WHERE c2.name = r.base AND c2.id <> r.id
      ) THEN r.base
      ELSE r.base || '-' || r.id::text
    END AS new_name
  FROM ranked r
)
UPDATE p2rms15pepb5ciz.customers c
SET
  raw_name = COALESCE(NULLIF(btrim(c.raw_name),''), resolved.old_name),
  name = resolved.new_name,
  updated_at = now()
FROM resolved
WHERE c.id = resolved.id;

COMMIT;

\echo '--- done ---'
