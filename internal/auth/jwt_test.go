package auth

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	m := NewManager("test-secret", time.Hour)
	token, err := m.Sign(1, "admin", "admin", []int64{1, 2}, true)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := m.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Username != "admin" || !claims.AllSites {
		t.Errorf("claims mismatch: %+v", claims)
	}
	if !m.IsAdmin(claims) {
		t.Error("admin role should pass IsAdmin")
	}
	if !m.CanAccessSite(claims, 999) {
		t.Error("all_sites should access any site")
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	m := NewManager("secret", time.Hour)
	if _, err := m.Verify("not-a-jwt"); err == nil {
		t.Error("expected error for invalid token")
	}

	// 篡改签名
	m2 := NewManager("other-secret", time.Hour)
	tok, _ := m2.Sign(1, "u", "viewer", nil, false)
	if _, err := m.Verify(tok); err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestRolePermissions(t *testing.T) {
	m := NewManager("secret", time.Hour)

	viewerTok, _ := m.Sign(2, "v", "viewer", []int64{5}, false)
	viewerClaims, _ := m.Verify(viewerTok)

	if m.IsAdmin(viewerClaims) {
		t.Error("viewer should not be admin")
	}
	if !m.CanAccessSite(viewerClaims, 5) {
		t.Error("viewer should access own site")
	}
	if m.CanAccessSite(viewerClaims, 99) {
		t.Error("viewer should NOT access other site")
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword(hash, "secret123") {
		t.Error("correct password should match")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("wrong password should not match")
	}
}

func TestTokenExpiry(t *testing.T) {
	m := NewManager("secret", -time.Hour)
	tok, _ := m.Sign(1, "u", "viewer", nil, false)
	if _, err := m.Verify(tok); err == nil {
		t.Error("expired token should fail")
	}
}
