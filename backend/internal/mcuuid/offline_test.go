package mcuuid

import "testing"

func TestOfflinePlayerUUID_Steve(t *testing.T) {
	s, err := OfflinePlayerUUIDString("Steve")
	if err != nil {
		t.Fatal(err)
	}
	// Совпадает с java.util.UUID.nameUUIDFromBytes("OfflinePlayer:Steve".getBytes(UTF_8))
	const want = "5627dd98-e6be-3c21-b8a8-e92344183641"
	if s != want {
		t.Fatalf("got %q, want %q", s, want)
	}
}
