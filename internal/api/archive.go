package api

import "github.com/TTstack/tt/internal/model"

type ArchiveImportRequest struct {
	Pull model.ArchivePull `json:"pull"`
	Spec model.VolumeSpec  `json:"spec"`
}
