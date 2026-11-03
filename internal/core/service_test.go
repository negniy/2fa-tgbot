package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// fakeRepo — in-memory SecretRepository для тестов сервиса. Повторяет
// контракт Postgres-реализации: секрет адресуется парой (владелец, ID).
type fakeRepo struct {
	nextID  int64
	records map[int64]fakeRecord
}

type fakeRecord struct {
	owner Owner
	rec   SecretRecord
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{records: make(map[int64]fakeRecord)}
}

func (r *fakeRepo) Find(_ context.Context, owner Owner, id int64) (SecretRecord, error) {
	fr, ok := r.records[id]
	if !ok || fr.owner != owner {
		return SecretRecord{}, ErrNotFound
	}
	return fr.rec, nil
}

func (r *fakeRepo) Add(_ context.Context, owner Owner, tag, encrypted string) error {
	for _, fr := range r.records {
		if fr.owner == owner && fr.rec.Tag == tag {
			return ErrAlreadyExists
		}
	}
	r.nextID++
	r.records[r.nextID] = fakeRecord{owner: owner, rec: SecretRecord{
		SecretInfo: SecretInfo{ID: r.nextID, Tag: tag},
		Encrypted:  encrypted,
	}}
	return nil
}

func (r *fakeRepo) Delete(ctx context.Context, owner Owner, id int64) (string, error) {
	rec, err := r.Find(ctx, owner, id)
	if err != nil {
		return "", err
	}
	delete(r.records, id)
	return rec.Tag, nil
}

func (r *fakeRepo) List(_ context.Context, owner Owner) ([]SecretInfo, error) {
	var out []SecretInfo
	for _, fr := range r.records {
		if fr.owner == owner {
			out = append(out, fr.rec.SecretInfo)
		}
	}
	return out, nil
}

// fakeCrypter «шифрует» добавлением префикса — достаточно, чтобы проверить,
// что в репозиторий уходит не открытый текст.
type fakeCrypter struct{}

func (fakeCrypter) Encrypt(plain SecretValue) (string, error) { return "enc:" + plain.Reveal(), nil }

func (fakeCrypter) Decrypt(encrypted string) (SecretValue, error) {
	if len(encrypted) < 4 || encrypted[:4] != "enc:" {
		return SecretValue{}, errors.New("bad ciphertext")
	}
	return NewSecret(encrypted[4:]), nil
}

// fakeTOTP возвращает в качестве «кода» сам секрет.
type fakeTOTP struct{}

func (fakeTOTP) Generate(secret SecretValue) (string, error) { return "code-" + secret.Reveal(), nil }

func newTestService(repo SecretRepository) *SecretService {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewSecretService(repo, fakeCrypter{}, fakeTOTP{}, nil, log)
}

func TestServiceAddGetDelete(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newTestService(repo)
	owner := NewUserOwner(1)

	if err := svc.AddSecret(ctx, owner, "github", NewSecret("S1")); err != nil {
		t.Fatal(err)
	}
	if got := repo.records[1].rec.Encrypted; got != "enc:S1" {
		t.Fatalf("repo stored %q, want encrypted value", got)
	}

	list, err := svc.ListSecrets(ctx, owner)
	if err != nil || len(list) != 1 || list[0].Tag != "github" {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	id := list[0].ID

	res, err := svc.GetCode(ctx, owner, id)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tag != "github" || res.Code != "code-S1" {
		t.Fatalf("unexpected code result: %+v", res)
	}
	if res.SecondsRemaining < 1 || res.SecondsRemaining > 30 {
		t.Fatalf("seconds remaining out of range: %d", res.SecondsRemaining)
	}

	tag, err := svc.DeleteSecret(ctx, owner, id)
	if err != nil || tag != "github" {
		t.Fatalf("delete: tag=%q err=%v", tag, err)
	}
	if _, err := svc.GetCode(ctx, owner, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

// TestServiceForeignIDNotFound: ID чужого секрета (например, из подделанных
// callback-данных) даёт ErrNotFound во всех операциях.
func TestServiceForeignIDNotFound(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newFakeRepo())
	alice, mallory := NewUserOwner(1), NewUserOwner(2)

	if err := svc.AddSecret(ctx, alice, "bank", NewSecret("S")); err != nil {
		t.Fatal(err)
	}
	const aliceSecretID = 1

	if _, err := svc.GetCode(ctx, mallory, aliceSecretID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetCode: err = %v", err)
	}
	if _, err := svc.GetSecretInfo(ctx, mallory, aliceSecretID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSecretInfo: err = %v", err)
	}
	if _, err := svc.DeleteSecret(ctx, mallory, aliceSecretID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteSecret: err = %v", err)
	}
	if _, err := svc.GetCode(ctx, alice, aliceSecretID); err != nil {
		t.Fatalf("owner must keep access: %v", err)
	}
}

func TestServiceAddDuplicate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newFakeRepo())
	owner := NewUserOwner(1)

	_ = svc.AddSecret(ctx, owner, "gh", NewSecret("A"))
	if err := svc.AddSecret(ctx, owner, "gh", NewSecret("B")); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
}
