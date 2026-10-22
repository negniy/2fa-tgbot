-- 0001_init.down.sql
-- Откат: возвращает telegram_id в secret, снимает owner-колонки, удаляет
-- users/identities. ВНИМАНИЕ: групповые секреты (owner_kind='group') при
-- откате теряют владельца — они не выражаются через telegram_id. Откат
-- предназначен для среды разработки, не для прод-данных с группами.

BEGIN;

ALTER TABLE secret ADD COLUMN IF NOT EXISTS telegram_id BIGINT;

-- Возвращаем telegram_id из identity для user-владельцев.
UPDATE secret s
SET telegram_id = i.provider_uid::bigint
FROM identities i
WHERE s.owner_kind = 'user'
  AND i.user_id = s.owner_id
  AND i.provider = 'telegram';

DROP INDEX IF EXISTS uq_secret_owner_tag;
ALTER TABLE secret DROP COLUMN IF EXISTS owner_kind;
ALTER TABLE secret DROP COLUMN IF EXISTS owner_id;

DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS users;

COMMIT;
