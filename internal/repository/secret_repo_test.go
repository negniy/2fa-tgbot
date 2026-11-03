package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"2fa-tgbot/internal/config"
	"2fa-tgbot/internal/core"
)

// Интеграционные тесты на настоящем Postgres. Запускаются, только если
// задан TEST_DATABASE_URL, например:
//
//	docker run --rm -d -p 127.0.0.1:55432:5432 -e POSTGRES_PASSWORD=test postgres:17
//	TEST_DATABASE_URL="postgres://postgres:test@127.0.0.1:55432/postgres?sslmode=disable" go test ./internal/repository/
//
// ВНИМАНИЕ: тесты очищают таблицы — не указывайте рабочую БД.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := config.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if _, err := db.Exec(`TRUNCATE secret, identities, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func TestSecretRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewSecretRepo(openTestDB(t))
	owner := core.NewUserOwner(1)

	for _, tag := range []string{"github", "aws"} {
		if err := repo.Add(ctx, owner, tag, "enc-"+tag); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.Add(ctx, owner, "github", "x"); !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("duplicate: err = %v", err)
	}

	list, err := repo.List(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Tag != "aws" || list[1].Tag != "github" {
		t.Fatalf("list not sorted by tag: %+v", list)
	}

	rec, err := repo.Find(ctx, owner, list[1].ID)
	if err != nil || rec.Tag != "github" || rec.Encrypted != "enc-github" {
		t.Fatalf("find: %+v err=%v", rec, err)
	}

	tag, err := repo.Delete(ctx, owner, rec.ID)
	if err != nil || tag != "github" {
		t.Fatalf("delete: tag=%q err=%v", tag, err)
	}
	if _, err := repo.Find(ctx, owner, rec.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("find after delete: err = %v", err)
	}
	if _, err := repo.Delete(ctx, owner, rec.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("second delete: err = %v", err)
	}
}

// TestSecretRepoOwnerIsolation: чужой ID недоступен ни на чтение, ни на
// удаление; user и group с одинаковым числовым ID не пересекаются.
func TestSecretRepoOwnerIsolation(t *testing.T) {
	ctx := context.Background()
	repo := NewSecretRepo(openTestDB(t))
	user, group, other := core.NewUserOwner(7), core.NewGroupOwner(7), core.NewUserOwner(8)

	if err := repo.Add(ctx, user, "bank", "enc"); err != nil {
		t.Fatal(err)
	}
	list, _ := repo.List(ctx, user)
	id := list[0].ID

	for _, o := range []core.Owner{group, other} {
		if _, err := repo.Find(ctx, o, id); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("find as %+v: err = %v", o, err)
		}
		if _, err := repo.Delete(ctx, o, id); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("delete as %+v: err = %v", o, err)
		}
		if l, _ := repo.List(ctx, o); len(l) != 0 {
			t.Fatalf("list as %+v leaked: %+v", o, l)
		}
	}
	if _, err := repo.Find(ctx, user, id); err != nil {
		t.Fatalf("owner lost access: %v", err)
	}
}
