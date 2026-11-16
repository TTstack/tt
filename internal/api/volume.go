package api

import "github.com/TTstack/tt/internal/model"

type VolumePage struct {
	Volumes     model.List[model.Volume] `json:"volumes"`
	NextToken   string                   `json:"next_token,omitempty"`
	MaxPageSize int                      `json:"max_page_size"`
}
type VolumeSnapshotPage struct {
	Snapshots   model.List[model.VolumeSnapshot] `json:"snapshots"`
	NextToken   string                           `json:"next_token,omitempty"`
	MaxPageSize int                              `json:"max_page_size"`
}
type SnapshotVolumeRequest struct {
	ID string `json:"id"`
}

func (v *SnapshotVolumeRequest) UnmarshalJSON(data []byte) error {
	type plain SnapshotVolumeRequest
	var next plain
	if err := model.DecodeRecord(data, &next, []string{"id"}, false); err != nil {
		return err
	}
	*v = SnapshotVolumeRequest(next)
	return nil
}
