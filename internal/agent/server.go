package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TTstack/tt/internal/api"
	"github.com/TTstack/tt/internal/auth"
	"github.com/TTstack/tt/internal/backup"
	"github.com/TTstack/tt/internal/capability"
	"github.com/TTstack/tt/internal/model"
	"github.com/TTstack/tt/internal/storage"
)

type imageCatalog struct {
	images  []string
	sizes   map[string]uint32
	warning *string
}
type Server struct {
	runtime           *Runtime
	mutation          sync.Mutex
	dbPath            string
	info              api.AgentInfo
	backupEnabled     bool
	backupUnsupported *string
	imageDir          string
	imagesMu          sync.RWMutex
	images            imageCatalog
	workCtx           context.Context
	catalogStore      func() (storage.ImageStore, error)
	catalogSlowAfter  time.Duration
	prepareRestore    func(context.Context, *model.Vm) error
}

func newServer(r *Runtime, path string) *Server {
	return &Server{runtime: r, dbPath: path, info: r.AgentInfo(), backupEnabled: r.backupEnabled, backupUnsupported: r.backupUnsupported, imageDir: r.imageDir, images: imageCatalog{images: []string{}, sizes: map[string]uint32{}, warning: stringPointer("image catalog refresh pending")}, workCtx: context.Background(), catalogStore: func() (storage.ImageStore, error) { return storage.New(r.storage) }, catalogSlowAfter: 5 * time.Second, prepareRestore: prepareDiskRestore}
}
func (s *Server) Handler(key *string) http.Handler {
	routes := []api.Route{
		{Pattern: "/api/info", Methods: []string{"GET"}, Handler: s.getInfo},
		{Pattern: "/api/images", Methods: []string{"GET"}, Handler: s.listImages},
		{Pattern: "/api/vms", Methods: []string{"GET", "POST"}, Handler: s.vms},
		{Pattern: "/api/vms/{id}", Methods: []string{"GET", "DELETE"}, Handler: s.vm},
		{Pattern: "/api/vms/{id}/stop", Methods: []string{"POST"}, Handler: s.stopVM},
		{Pattern: "/api/vms/{id}/start", Methods: []string{"POST"}, Handler: s.startVM},
		{Pattern: "/api/vms/{id}/resources", Methods: []string{"POST"}, Handler: s.resizeVM},
		{Pattern: "/api/vms/{id}/backup", Methods: []string{"GET", "PUT", "DELETE"}, Handler: s.backupVM},
		{Pattern: "/api/vms/{id}/backup/restore", Methods: []string{"POST"}, Handler: s.restoreBackup},
	}
	return api.StripHeadBody(api.Router(routes, func(next http.Handler) http.Handler { return auth.Middleware(key, next) }))
}

// Request cancellation never supplies the mutation's context. net/http keeps
// this handler alive after a disconnect; service shutdown drains active handlers
// before releasing the runtime state lock.
func mutate[T any](s *Server, f func(context.Context, *Runtime) (T, error)) (T, error) {
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return f(s.workCtx, s.runtime)
}
func failure(w http.ResponseWriter, err error) {
	api.WriteJSON(w, 500, api.Failure[model.Unit](err.Error()))
}
func actionReply(w http.ResponseWriter, err error) {
	if err != nil {
		failure(w, err)
	} else {
		api.WriteJSON(w, 200, api.Empty())
	}
}
func (s *Server) getInfo(w http.ResponseWriter, request *http.Request) {
	vms, corrupt, err := ReadRecoverySnapshot(request.Context(), s.dbPath, s.runtime.db.stderr)
	if err != nil {
		failure(w, err)
		return
	}
	info := s.info
	info.Resource = ResourcesFor(info.Resource, vms)
	info.Warnings = append(model.List[string]{}, info.Warnings...)
	if corrupt {
		info.Resource.CPUUsed = info.Resource.CPUTotal
		info.Resource.MemUsed = info.Resource.MemTotal
		info.Resource.DiskUsed = info.Resource.DiskTotal
		info.Warnings = append(info.Warnings, "unreadable VM records retained; new admission disabled; inspect agent logs")
	}
	s.imagesMu.RLock()
	info.Images = append(model.List[string]{}, s.images.images...)
	info.ImageSizes = s.images.sizes
	if s.images.warning != nil {
		info.Warnings = append(info.Warnings, *s.images.warning)
	}
	s.imagesMu.RUnlock()
	inventory := model.List[model.Vm](vms)
	info.VMs = &inventory
	api.WriteJSON(w, 200, api.Success(info))
}
func (s *Server) listImages(w http.ResponseWriter, _ *http.Request) {
	s.imagesMu.RLock()
	images := append(model.List[string]{}, s.images.images...)
	s.imagesMu.RUnlock()
	api.WriteJSON(w, 200, api.Success(images))
}
func (s *Server) vms(w http.ResponseWriter, request *http.Request) {
	if request.Method == "GET" || request.Method == "HEAD" {
		vms, err := ReadVMs(request.Context(), s.dbPath)
		if err != nil {
			failure(w, err)
		} else {
			api.WriteJSON(w, 200, api.Success(model.List[model.Vm](vms)))
		}
		return
	}
	body, ok := api.ReadJSON[api.CreateVmReq](w, request)
	if !ok {
		return
	}
	vm, err := mutate(s, func(ctx context.Context, r *Runtime) (*model.Vm, error) { return r.CreateVM(ctx, body) })
	if err != nil {
		if strings.Contains(err.Error(), "capability ") {
			api.WriteJSON(w, 400, api.Failure[model.Unit](err.Error()))
		} else {
			failure(w, err)
		}
		return
	}
	api.WriteJSON(w, 201, api.Success(api.CreateVmResp{Vm: *vm}))
}
func (s *Server) vm(w http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if request.Method == "DELETE" {
		_, err := mutate(s, func(ctx context.Context, r *Runtime) (model.Unit, error) { return model.Unit{}, r.DestroyVM(ctx, id) })
		actionReply(w, err)
		return
	}
	vm, err := ReadVM(request.Context(), s.dbPath, id)
	if err != nil {
		failure(w, err)
	} else if vm == nil {
		api.WriteJSON(w, 404, api.Failure[model.Vm]("VM not found: "+id))
	} else {
		api.WriteJSON(w, 200, api.Success(*vm))
	}
}
func (s *Server) stopVM(w http.ResponseWriter, request *http.Request) {
	_, err := mutate(s, func(ctx context.Context, r *Runtime) (model.Unit, error) {
		return model.Unit{}, r.StopVM(ctx, request.PathValue("id"))
	})
	actionReply(w, err)
}
func (s *Server) startVM(w http.ResponseWriter, request *http.Request) {
	_, err := mutate(s, func(ctx context.Context, r *Runtime) (model.Unit, error) {
		return model.Unit{}, r.StartVM(ctx, request.PathValue("id"))
	})
	actionReply(w, err)
}
func (s *Server) resizeVM(w http.ResponseWriter, request *http.Request) {
	target, ok := api.ReadJSON[model.VmResources](w, request)
	if !ok {
		return
	}
	vm, err := mutate(s, func(ctx context.Context, r *Runtime) (*model.Vm, error) {
		return r.ResizeVM(ctx, request.PathValue("id"), target)
	})
	if err == nil {
		api.WriteJSON(w, 200, api.Success(*vm))
		return
	}
	message := err.Error()
	status := 500
	if strings.Contains(message, "insufficient resources") || strings.Contains(message, "require a confirmed stopped VM") || strings.Contains(message, "retry the recorded target first") {
		status = 409
	} else if strings.Contains(message, "shrinking") || strings.Contains(message, "must be > 0") || strings.Contains(message, "capability ") || strings.Contains(message, "overflow") {
		status = 400
	}
	api.WriteJSON(w, status, api.Failure[model.Unit](message))
}
func backupReply(w http.ResponseWriter, view api.BackupView, err error) {
	if err != nil {
		api.WriteJSON(w, backup.ErrorStatus(err.Error()), api.Failure[model.Unit](err.Error()))
		return
	}
	w.Header().Set("ETag", "\""+view.Vm.Backup.Revision+"\"")
	api.WriteJSON(w, 200, api.Success(view))
}
func (s *Server) backupVM(w http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if request.Method == "GET" || request.Method == "HEAD" {
		vm, err := ReadVM(request.Context(), s.dbPath, id)
		if err != nil {
			backupReply(w, api.BackupView{}, err)
			return
		}
		if vm == nil {
			backupReply(w, api.BackupView{}, fmt.Errorf("not found: VM %s", id))
			return
		}
		reason := capability.BackupUnsupported(vm.Engine, s.info.Storage, s.backupUnsupported, s.info.CapabilityReport)
		backupReply(w, api.BackupView{Vm: *vm, Enabled: s.backupEnabled, Supported: reason == nil, UnsupportedReason: reason}, nil)
		return
	}
	action := model.BackupCreate
	if request.Method == "DELETE" {
		action = model.BackupDelete
	}
	s.backupRequest(w, request, action, nil)
}
func (s *Server) restoreBackup(w http.ResponseWriter, request *http.Request) {
	body, ok := api.ReadJSON[api.RestoreBody](w, request)
	if !ok {
		return
	}
	s.backupRequest(w, request, model.BackupRestore, &body.Generation)
}
func (s *Server) backupRequest(w http.ResponseWriter, request *http.Request, action model.BackupAction, generation *string) {
	id := request.PathValue("id")
	identity, err := backup.FromHeaders(api.HeaderText(request.Header, "Idempotency-Key"), api.HeaderText(request.Header, "If-Match"), action, generation)
	if err != nil {
		backupReply(w, api.BackupView{}, err)
		return
	}
	view, err := s.runBackup(id, identity, true)
	if err != nil {
		s.mutation.Lock()
		sequence := s.runtime.BackupSettled(s.workCtx, id, identity)
		s.mutation.Unlock()
		if sequence != nil {
			w.Header().Set("x-tt-backup-settled", identity.OperationID)
			w.Header().Set("x-tt-backup-settled-sequence", fmt.Sprint(*sequence))
		}
	}
	backupReply(w, view, err)
}
func (s *Server) RefreshImages(ctx context.Context) {
	result := make(chan imageCatalog, 1)
	go func() {
		store, err := s.catalogStore()
		catalog := imageCatalog{images: []string{}, sizes: map[string]uint32{}}
		if err == nil {
			catalog.images, err = store.ListImages(ctx, s.imageDir)
		}
		if err == nil {
			catalog.sizes = storage.ImageSizes(ctx, store, s.imageDir, catalog.images)
		} else {
			fmt.Fprintf(s.runtime.db.stderr, "[agent] image catalog unavailable: %v\n", err)
			catalog.images = []string{}
			catalog.warning = stringPointer("image catalog unavailable: " + err.Error())
		}
		result <- catalog
	}()
	timer := time.NewTimer(s.catalogSlowAfter)
	defer timer.Stop()
	var catalog imageCatalog
	select {
	case catalog = <-result:
	case <-timer.C:
		s.imagesMu.Lock()
		s.images.warning = stringPointer("image catalog refresh is slow; serving cached images")
		s.imagesMu.Unlock()
		catalog = <-result
	}
	s.imagesMu.Lock()
	s.images = catalog
	s.imagesMu.Unlock()
}
func (s *Server) ReconcileOnce(ctx context.Context) error {
	vms, _, err := ReadRecoverySnapshot(ctx, s.dbPath, s.runtime.db.stderr)
	if err != nil {
		return err
	}
	for _, vm := range vms {
		_, err := mutate(s, func(ctx context.Context, r *Runtime) (model.Unit, error) {
			return model.Unit{}, r.ReconcileVM(ctx, vm.ID)
		})
		if err != nil {
			fmt.Fprintf(s.runtime.db.stderr, "[agent] reconcile %s: %v\n", vm.ID, err)
		}
		if vm.Backup.Pending != nil {
			if _, err := s.runBackup(vm.ID, vm.Backup.Pending.Request, false); err != nil {
				fmt.Fprintf(s.runtime.db.stderr, "[agent] backup recovery %s: %v\n", vm.ID, err)
			}
		} else if len(vm.Backup.Retired) > 0 {
			s.cleanupBackup(vm.ID)
		}
	}
	return nil
}
