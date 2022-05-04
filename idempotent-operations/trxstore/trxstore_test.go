package trxstore

import (
	"bytes"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestNewTRXStore(t *testing.T) {
	trx := uuid.New()
	store := NewBytesTRXStore(10 * time.Millisecond)
	res := store.Check(trx)
	if res != nil {
		t.Fatalf("new trx already exists")
	}

	expected := []byte("test")
	store.Store(trx, expected)
	res = store.Check(trx)
	if !bytes.Equal(res, expected) {
		t.Fatalf("trx not exists")
	}

	time.Sleep(1 * time.Second)
	res = store.Check(trx)
	if res != nil {
		t.Fatalf("trx not expired")
	}
}
