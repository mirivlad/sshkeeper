package syncer

// Stamp gives every current local record the time of its last change and adds
// deletions for records that disappeared since the last sync.
//
// A record whose hash matches the last synced state keeps that state's time.
// A changed or new record gets now. A record never synced from this device but
// already present remotely is treated as older than the remote copy (time 0),
// so joining a sync space adopts its data instead of overwriting it.
func Stamp(current []Record, states map[string]Record, remote map[string]Record, now int64) []Record {
	stamped := make([]Record, 0, len(current)+len(states))
	seen := map[string]bool{}
	for _, record := range current {
		seen[record.ID] = true
		previous, synced := states[record.ID]
		switch {
		case synced && !previous.Deleted && previous.Hash == record.Hash:
			record.Updated = previous.Updated
		case !synced && remote[record.ID].ID != "" && remote[record.ID].Hash == record.Hash:
			record.Updated = remote[record.ID].Updated
		case !synced && remote[record.ID].ID != "":
			record.Updated = 0
		default:
			record.Updated = now
		}
		stamped = append(stamped, record)
	}
	for id, previous := range states {
		if seen[id] {
			continue
		}
		deletion := Record{ID: id, Kind: KindOf(id), Deleted: true, Updated: previous.Updated}
		if !previous.Deleted {
			deletion.Updated = now
		}
		stamped = append(stamped, deletion)
	}
	SortRecords(stamped)
	return stamped
}

// MergeResult is the outcome of merging local and remote records.
type MergeResult struct {
	// Records is the merged state to store remotely and remember locally.
	Records []Record
	// Incoming are the remote versions that won over the local ones and must
	// be applied to this device.
	Incoming []Record
	// Outgoing counts local versions that won over the remote ones.
	Outgoing int
}

// Merge resolves every record by the latest change. Ties go to the version
// with the greater hash so all devices reach the same result.
func Merge(local []Record, remote []Record) MergeResult {
	localByID := map[string]Record{}
	for _, record := range local {
		localByID[record.ID] = record
	}
	remoteByID := map[string]Record{}
	for _, record := range remote {
		remoteByID[record.ID] = record
	}

	result := MergeResult{}
	ids := map[string]bool{}
	for id := range localByID {
		ids[id] = true
	}
	for id := range remoteByID {
		ids[id] = true
	}
	for id := range ids {
		mine, haveMine := localByID[id]
		theirs, haveTheirs := remoteByID[id]
		switch {
		case !haveTheirs:
			result.Records = append(result.Records, mine)
			if !mine.Deleted {
				result.Outgoing++
			}
		case !haveMine:
			result.Records = append(result.Records, theirs)
			if !theirs.Deleted {
				result.Incoming = append(result.Incoming, theirs)
			}
		case sameVersion(mine, theirs):
			result.Records = append(result.Records, theirs)
		case newer(theirs, mine):
			result.Records = append(result.Records, theirs)
			// A deletion of something this device never had needs no work.
			if !(theirs.Deleted && mine.Deleted) {
				result.Incoming = append(result.Incoming, theirs)
			}
		default:
			result.Records = append(result.Records, mine)
			result.Outgoing++
		}
	}
	SortRecords(result.Records)
	SortRecords(result.Incoming)
	return result
}

func sameVersion(a, b Record) bool {
	return a.Deleted == b.Deleted && a.Hash == b.Hash
}

func newer(a, b Record) bool {
	if a.Updated != b.Updated {
		return a.Updated > b.Updated
	}
	if a.Deleted != b.Deleted {
		// A deletion and an edit at the same instant: keep the data.
		return !a.Deleted
	}
	return a.Hash > b.Hash
}
