-- 0001_init.up.sql
-- Вводит модель users/identities и переводит secret с telegram_id на
-- владельца (owner_kind, owner_id), перенося существующие данные.
--
-- Безопасна к повторному запуску: IF [NOT] EXISTS + проверка наличия
-- legacy-колонки telegram_id. Перенос данных детерминированный (через
-- временную таблицу-маппинг), без «угадывания» соответствия.

BEGIN;

-- 1. Пользователи (ядро идентичности).
CREATE TABLE IF NOT EXISTS users (
    id             BIGSERIAL PRIMARY KEY,
    email          TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. Способы входа, привязанные к пользователю.
CREATE TABLE IF NOT EXISTS identities (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider       TEXT NOT NULL,
    provider_uid   TEXT NOT NULL,
    password_hash  TEXT,
    email          TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);

CREATE INDEX IF NOT EXISTS idx_identities_user_id ON identities(user_id);

-- 2.5. Таблица secret на свежей БД (раньше создавалась в коде).
CREATE TABLE IF NOT EXISTS secret (
    id     BIGSERIAL PRIMARY KEY,
    tag    TEXT NOT NULL,
    secret TEXT NOT NULL
);

-- 3. Новые колонки владельца в secret.
ALTER TABLE secret ADD COLUMN IF NOT EXISTS owner_kind TEXT;
ALTER TABLE secret ADD COLUMN IF NOT EXISTS owner_id   BIGINT;

-- 4. Перенос legacy telegram_id -> users/identities/owner.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'secret' AND column_name = 'telegram_id'
    ) THEN
        -- 4a. Маппинг telegram_id -> user_id. Для уже существующих
        --     telegram-identity берём их user_id; для новых создаём users.
        CREATE TEMP TABLE tg_map (
            telegram_id BIGINT PRIMARY KEY,
            user_id     BIGINT
        ) ON COMMIT DROP;

        INSERT INTO tg_map (telegram_id)
        SELECT DISTINCT telegram_id
        FROM secret
        WHERE telegram_id IS NOT NULL;

        -- Уже привязанные telegram_id.
        UPDATE tg_map m
        SET user_id = i.user_id
        FROM identities i
        WHERE i.provider = 'telegram'
          AND i.provider_uid = m.telegram_id::text;

        -- Для оставшихся создаём users по одному и фиксируем id.
        DECLARE
            r RECORD;
            new_uid BIGINT;
        BEGIN
            FOR r IN SELECT telegram_id FROM tg_map WHERE user_id IS NULL LOOP
                INSERT INTO users (created_at) VALUES (now()) RETURNING id INTO new_uid;
                INSERT INTO identities (user_id, provider, provider_uid)
                VALUES (new_uid, 'telegram', r.telegram_id::text);
                UPDATE tg_map SET user_id = new_uid WHERE telegram_id = r.telegram_id;
            END LOOP;
        END;

        -- 4b. Проставляем owner в secret по маппингу.
        UPDATE secret s
        SET owner_kind = 'user',
            owner_id   = m.user_id
        FROM tg_map m
        WHERE s.telegram_id = m.telegram_id
          AND s.owner_id IS NULL;
    END IF;
END $$;

-- 5. owner-колонки обязательны после переноса.
ALTER TABLE secret ALTER COLUMN owner_kind SET NOT NULL;
ALTER TABLE secret ALTER COLUMN owner_id   SET NOT NULL;

-- 6. Уникальность имени сервиса в рамках владельца (чинит дубли).
CREATE UNIQUE INDEX IF NOT EXISTS uq_secret_owner_tag
    ON secret (owner_kind, owner_id, tag);

-- 7. Удаляем legacy-колонку.
ALTER TABLE secret DROP COLUMN IF EXISTS telegram_id;

COMMIT;
