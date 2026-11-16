package api

import "github.com/TTstack/tt/internal/model"

type CheckpointRequest struct {
	ID string `json:"id"`
}

func (v *CheckpointRequest) UnmarshalJSON(data []byte) error {
	type plain CheckpointRequest
	var next plain
	if err := model.DecodeRecord(data, &next, []string{"id"}, false); err != nil {
		return err
	}
	*v = CheckpointRequest(next)
	return nil
}

type RecoveryPointPage struct {
	Points      model.List[model.RecoveryPoint] `json:"points"`
	NextToken   string                          `json:"next_token,omitempty"`
	MaxPageSize int                             `json:"max_page_size"`
}
type RestorePointRequest struct {
	PointID     string            `json:"point_id"`
	Owner       string            `json:"owner"`
	CPU         *uint32           `json:"cpu,omitempty"`
	Mem         *uint32           `json:"mem,omitempty"`
	GuestConfig model.GuestConfig `json:"guest_config,omitempty"`
}

func (v *RestorePointRequest) UnmarshalJSON(data []byte) error {
	type plain RestorePointRequest
	var next plain
	if err := model.DecodeRecord(data, &next, []string{"point_id", "owner"}, false); err != nil {
		return err
	}
	*v = RestorePointRequest(next)
	return nil
}
