package auth

import "testing"

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !IsArgon2IDHash(hash) {
		t.Fatalf("HashPassword() = %q, want argon2id hash", hash)
	}
	ok, err := VerifyPassword(hash, "hunter2")
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword() = false, want true")
	}
	ok, err = VerifyPassword(hash, "wrong")
	if err != nil {
		t.Fatalf("VerifyPassword(wrong) error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword(wrong) = true, want false")
	}
}
