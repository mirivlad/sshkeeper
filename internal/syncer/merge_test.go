package syncer

import (
	"strings"
	"testing"
)

func rec(t *testing.T, key, name string, updated int64) Record {
	t.Helper()
	record, err := NewRecord(KindGroup, key, NameData{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	record.Updated = updated
	return record
}

func ids(records []Record) string {
	parts := make([]string, len(records))
	for index, record := range records {
		parts[index] = record.ID
		if record.Deleted {
			parts[index] += "(deleted)"
		}
	}
	return strings.Join(parts, ",")
}

func TestStampKeepsUnchangedTimesAndMarksChanges(t *testing.T) {
	same := rec(t, "a", "A", 0)
	changed := rec(t, "b", "B2", 0)
	fresh := rec(t, "c", "C", 0)
	states := map[string]Record{
		"group:a":            {ID: "group:a", Hash: same.Hash, Updated: 10},
		"group:b":            {ID: "group:b", Hash: rec(t, "b", "B1", 0).Hash, Updated: 10},
		"group:gone":         {ID: "group:gone", Hash: "x", Updated: 10},
		"group:old-deletion": {ID: "group:old-deletion", Deleted: true, Updated: 5},
	}
	stamped := Stamp([]Record{same, changed, fresh}, states, nil, 100)
	got := map[string]Record{}
	for _, record := range stamped {
		got[record.ID] = record
	}
	if got["group:a"].Updated != 10 || got["group:b"].Updated != 100 || got["group:c"].Updated != 100 {
		t.Fatalf("unexpected times: %+v", stamped)
	}
	if !got["group:gone"].Deleted || got["group:gone"].Updated != 100 {
		t.Fatalf("a record removed since the last sync must become a deletion now: %+v", got["group:gone"])
	}
	if !got["group:old-deletion"].Deleted || got["group:old-deletion"].Updated != 5 {
		t.Fatalf("an old deletion keeps its time: %+v", got["group:old-deletion"])
	}
}

func TestStampAdoptsRemoteOnFirstSync(t *testing.T) {
	local := rec(t, "a", "local version", 0)
	remote := map[string]Record{"group:a": rec(t, "a", "remote version", 50)}
	stamped := Stamp([]Record{local}, nil, remote, 100)
	if stamped[0].Updated != 0 {
		t.Fatalf("an unsynced local copy must lose to the remote one, got %d", stamped[0].Updated)
	}
	result := Merge(stamped, []Record{remote["group:a"]})
	if len(result.Incoming) != 1 || result.Incoming[0].Hash != remote["group:a"].Hash {
		t.Fatalf("remote version should be applied: %+v", result)
	}
}

func TestMergePicksLatestChangeEachWay(t *testing.T) {
	local := []Record{rec(t, "mine-newer", "L", 20), rec(t, "theirs-newer", "L", 10), rec(t, "only-local", "L", 5)}
	remote := []Record{rec(t, "mine-newer", "R", 10), rec(t, "theirs-newer", "R", 20), rec(t, "only-remote", "R", 5)}
	result := Merge(local, remote)
	if got := ids(result.Incoming); got != "group:only-remote,group:theirs-newer" {
		t.Fatalf("incoming = %s", got)
	}
	if result.Outgoing != 2 {
		t.Fatalf("outgoing = %d, want mine-newer and only-local", result.Outgoing)
	}
	if got := ids(result.Records); got != "group:mine-newer,group:only-local,group:only-remote,group:theirs-newer" {
		t.Fatalf("merged = %s", got)
	}
}

func TestMergeDeletionsWinOnlyWhenNewer(t *testing.T) {
	edited := rec(t, "a", "edited", 30)
	deletedEarlier := Record{ID: "group:a", Kind: KindGroup, Deleted: true, Updated: 20}
	if result := Merge([]Record{edited}, []Record{deletedEarlier}); len(result.Incoming) != 0 || result.Records[0].Deleted {
		t.Fatalf("a later edit must survive an earlier deletion: %+v", result)
	}
	deletedLater := Record{ID: "group:a", Kind: KindGroup, Deleted: true, Updated: 40}
	result := Merge([]Record{edited}, []Record{deletedLater})
	if len(result.Incoming) != 1 || !result.Incoming[0].Deleted {
		t.Fatalf("a later deletion must reach this device: %+v", result)
	}
}

func TestMergeIsDeterministicOnTies(t *testing.T) {
	a := rec(t, "x", "A", 10)
	b := rec(t, "x", "B", 10)
	first := Merge([]Record{a}, []Record{b}).Records[0].Hash
	second := Merge([]Record{b}, []Record{a}).Records[0].Hash
	if first != second {
		t.Fatal("both devices must agree on a tie")
	}
}
