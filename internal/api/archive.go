package api

import "github.com/TTstack/tt/internal/model"

type ArchiveImportRequest struct {
	Pull model.ArchivePull `json:"pull"`
	Spec model.VolumeSpec  `json:"spec"`
}

func (v *ArchiveImportRequest) UnmarshalJSON(data []byte) error {
	type plain ArchiveImportRequest
	var next plain
	if err := model.DecodeRecord(data, &next, []string{"pull", "spec"}, false); err != nil {
		return err
	}
	*v = ArchiveImportRequest(next)
	return nil
}
