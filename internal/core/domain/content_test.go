package domain

import "testing"

// Saints and apparitions each take two uploads: a wide one for the top of the
// page and a square one for thumbnails.
func TestSaintsAndApparitionsHaveWideAndSquareImages(t *testing.T) {
	for _, key := range []string{"saints", "apparitions"} {
		var found *Collection
		for i := range Collections {
			if Collections[i].Key == key {
				found = &Collections[i]
			}
		}
		if found == nil {
			t.Fatalf("collection %s missing", key)
		}
		for _, field := range []string{"artworkURL", "wideArtworkURL"} {
			ok := false
			for _, f := range found.Fields {
				if f.Key == field {
					ok = f.Type == FieldImage && !f.Required
				}
			}
			if !ok {
				t.Errorf("%s: %s should be an optional image field", key, field)
			}
		}
	}
}
