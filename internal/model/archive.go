package model

// DiskArchive is a portable, bounded image/disk transfer manifest. Archive paths
// are fixed format members, never host-local paths or caller-supplied bindings.
type DiskArchive struct {
	Version int                 `json:"version"`
	Engine  Engine              `json:"engine,omitempty"`
	Image   string              `json:"image,omitempty"`
	Format  string              `json:"format"`
	SizeMiB uint32              `json:"size_mib"`
	Members List[ArchiveMember] `json:"members"`
}
type ArchiveMember struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}
type ArchivePull struct {
	ID          string `json:"id"`
	SHA256      string `json:"sha256"`
	Bytes       int64  `json:"bytes"`
	ExpandedMiB uint32 `json:"expanded_mib"`
}

func (v *ArchivePull) UnmarshalJSON(data []byte) error {
	type plain ArchivePull
	var next plain
	if err := DecodeRecord(data, &next, []string{"id", "sha256", "bytes", "expanded_mib"}, false); err != nil {
		return err
	}
	*v = ArchivePull(next)
	return nil
}

func (v *DiskArchive) UnmarshalJSON(data []byte) error {
	type plain DiskArchive
	var next plain
	if err := DecodeRecord(data, &next, []string{"version", "format", "size_mib", "members"}, false); err != nil {
		return err
	}
	*v = DiskArchive(next)
	return nil
}

func (v *ArchiveMember) UnmarshalJSON(data []byte) error {
	type plain ArchiveMember
	var next plain
	if err := DecodeRecord(data, &next, []string{"name", "bytes"}, false); err != nil {
		return err
	}
	*v = ArchiveMember(next)
	return nil
}
