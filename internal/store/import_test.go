package store

import (
	"errors"
	"testing"
)

// exportDoc builds import items from existing stored data, as an export would.
func exportItems(t *testing.T, s *Store, noteIDs ...string) []ImportItem {
	t.Helper()
	var items []ImportItem
	for _, id := range noteIDs {
		n, err := s.GetNote(id)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, ImportItem{Note: &n})
		as, _ := s.Addenda(id)
		for i := range as {
			items = append(items, ImportItem{Addendum: &as[i]})
		}
	}
	return items
}

func TestImportRoundTripAndIdempotent(t *testing.T) {
	src, _ := newStore(t)
	sd := seed(t, src)
	items := exportItems(t, src, sd.note1, sd.note2)
	dst, _ := newStore(t)
	res, err := dst.Import(items)
	if err != nil || res.Inserted != 3 || res.Skipped != 0 {
		t.Fatalf("first import: %+v %v", res, err)
	}
	e, err := dst.Entry(sd.o1)
	if err != nil || !e.Closed || e.ClosedBy != sd.a1 {
		t.Fatalf("link state lost: %+v %v", e, err)
	}
	res, err = dst.Import(items)
	if err != nil || res.Inserted != 0 || res.Skipped != 3 {
		t.Fatalf("second import must skip everything: %+v %v", res, err)
	}
	if n, _ := dst.CountNotes(); n != 2 {
		t.Fatalf("notes = %d", n)
	}
}

func TestImportOrdersByDependency(t *testing.T) {
	src, _ := newStore(t)
	sd := seed(t, src)
	items := exportItems(t, src, sd.note1, sd.note2)
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 { // addendum before its note
		items[i], items[j] = items[j], items[i]
	}
	dst, _ := newStore(t)
	if res, err := dst.Import(items); err != nil || res.Inserted != 3 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestImportConflictAborts(t *testing.T) {
	src, _ := newStore(t)
	sd := seed(t, src)
	items := exportItems(t, src, sd.note1, sd.note2)
	dst, _ := newStore(t)
	if _, err := dst.Import(items[:1]); err != nil {
		t.Fatal(err)
	}
	tampered := *items[0].Note
	tampered.Title = "Rewritten history"
	batch := []ImportItem{{Note: &tampered}, items[len(items)-1]} // second item is a new, valid note
	if _, err := dst.Import(batch); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if n, _ := dst.CountNotes(); n != 1 {
		t.Fatalf("batch must be all-or-nothing, notes = %d", n)
	}
	got, _ := dst.GetNote(sd.note1)
	if got.Title != "Add tidal index" {
		t.Fatalf("existing note changed: %q", got.Title)
	}
}

func TestImportEntryIDCollisionAborts(t *testing.T) {
	src, _ := newStore(t)
	sd := seed(t, src)
	dst, _ := newStore(t)
	if _, err := dst.Import(exportItems(t, src, sd.note1)); err != nil {
		t.Fatal(err)
	}
	other := sampleNote()
	other.ID = "n-zzzzzz"
	other.Entries[0].ID = sd.change // an entry ID already used by another note
	if _, err := dst.Import([]ImportItem{{Note: &other}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestImportMissingDependencyFails(t *testing.T) {
	src, _ := newStore(t)
	sd := seed(t, src)
	items := exportItems(t, src, sd.note1)
	dst, _ := newStore(t)
	if _, err := dst.Import(items[1:]); err == nil { // the addendum without its note
		t.Fatal("addendum without its note must fail")
	}
	if n, _ := dst.CountNotes(); n != 0 {
		t.Fatalf("notes = %d", n)
	}
}
