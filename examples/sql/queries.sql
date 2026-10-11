-- Useful queries for bombaychitra.db.
--   sqlite3 -header -column data/build/bombaychitra.db < examples/sql/queries.sql

-- Dataset metadata
SELECT key, value FROM meta WHERE key IN ('region_name', 'built_at', 'source_timestamp', 'count_places');

-- Places per category group
SELECT g.label, SUM(c.count) AS places
FROM categories c JOIN category_groups g ON g.id = c.group_id
GROUP BY g.id ORDER BY places DESC;

-- Hospitals in a bounding box (Bandra), using the R*Tree index
SELECT p.name, p.phone, p.locality
FROM places_rtree r JOIN places p ON p.id = r.id
WHERE r.min_lon >= 72.82 AND r.max_lon <= 72.86 AND r.min_lat >= 19.04 AND r.max_lat <= 19.08
  AND p.category = 'hospital' AND p.name IS NOT NULL;

-- Schools and colleges per area
SELECT a.name AS area, COUNT(*) AS institutions
FROM area_members m
JOIN places p ON p.id = m.member_id AND m.kind = 'place'
JOIN areas a ON a.id = m.area_id
WHERE p.category IN ('school', 'college', 'university')
GROUP BY a.id ORDER BY institutions DESC;

-- Full-text search, ranked like the API
SELECT kind, name, detail
FROM search WHERE search MATCH '"bandra" "stat"*'
ORDER BY bm25(search, 10.0, 4.0, 1.0) * (1 - 0.15 * weight) LIMIT 10;

-- Longest named bridges by type
SELECT kind, name, ROUND(length_m / 1000.0, 2) AS km, locality
FROM named_bridges WHERE kind IN ('road', 'metro', 'rail')
ORDER BY length_m DESC LIMIT 15;

-- Tallest buildings
SELECT name, levels, height_m, locality
FROM buildings WHERE height_m IS NOT NULL AND name IS NOT NULL
ORDER BY height_m DESC LIMIT 15;

-- Stops of Metro Line 1 (Versova → Ghatkopar) in order
SELECT s.key + 1 AS seq, json_extract(s.value, '$.name') AS stop
FROM transit_lines t, json_each(t.stops) s
WHERE t.line_key = 'Mumbai Metro:1' AND t.from_name = 'Versova';

-- Longest named streets in Navi Mumbai
SELECT s.name, s.kind, ROUND(s.length_m / 1000.0, 1) AS km
FROM streets s JOIN area_members m ON m.member_id = s.id AND m.kind = 'street'
WHERE m.area_id = 'navi-mumbai' AND s.name IS NOT NULL
ORDER BY s.length_m DESC LIMIT 10;
