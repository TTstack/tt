package ctl

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TTstack/tt/internal/api"
	"github.com/TTstack/tt/internal/model"
)

func validAgentAddress(address string) bool {
	parsed, err := url.Parse("http://" + address)
	if err != nil {
		return false
	}
	if parsed.Hostname() == "" || parsed.Port() == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(address, "#") {
		return false
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	// url::Url removes an explicit default HTTP port; its port() is then None.
	return err == nil && port != 80
}
func (s *Server) registerHost(ctx context.Context, request api.RegisterHostReq) (*model.Host, int, *APIError) {
	if !validAgentAddress(request.Addr) {
		return nil, 0, apiError(400, "agent address must be host:port")
	}
	// A tolerated trailing slash must not survive into stored addresses: the
	// client joins paths by concatenation and would emit double-slash URLs.
	request.Addr = strings.TrimSuffix(request.Addr, "/")
	client, err := s.clientFactory(30 * time.Second)
	if err != nil {
		return nil, 0, internal(err)
	}
	response, err := client.request(ctx, "GET", request.Addr, "/api/info", nil, nil)
	if err != nil {
		return nil, 0, apiError(502, fmt.Sprintf("cannot reach agent at %s: %v", request.Addr, err))
	}
	status := response.StatusCode
	reply, err := decodeResponse[api.AgentInfo](response)
	if err != nil {
		return nil, 0, apiError(502, "invalid agent response: "+err.Error())
	}
	if !successful(status) || !reply.OK || reply.Data == nil {
		return nil, 0, apiError(502, "agent returned no data")
	}
	info := reply.Data
	if err := model.ValidateName(info.HostID, "host_id"); err != nil {
		return nil, 0, apiError(502, err.Error())
	}
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	existing, err := s.db.ListHosts(ctx)
	if err != nil {
		return nil, 0, internal(err)
	}
	for _, host := range existing {
		if (host.ID == info.HostID && host.Addr != request.Addr) || (host.Addr == request.Addr && host.ID != info.HostID) {
			return nil, 0, apiError(409, "host ID or address already belongs to a registered peer; drain and remove it before replacement")
		}
	}
	for _, host := range existing {
		if host.ID == info.HostID {
			previous := host.Resource
			host.Resource = info.Resource
			host.Resource.CPUUsed = max(host.Resource.CPUUsed, previous.CPUUsed)
			host.Resource.MemUsed = max(host.Resource.MemUsed, previous.MemUsed)
			host.Resource.DiskUsed = max(host.Resource.DiskUsed, previous.DiskUsed)
			host.Resource.VmCount = max(host.Resource.VmCount, previous.VmCount)
			host.Engines = info.Engines
			host.Capabilities = info.Capabilities
			host.CapabilityReport = info.CapabilityReport
			host.Images = info.Images
			host.ImageSizes = info.ImageSizes
			host.State = model.HostOnline
			host.Error = nil
			if len(info.Warnings) > 0 {
				message := strings.Join(info.Warnings, "; ")
				host.Error = &message
			}
			if err := s.db.PutHost(ctx, &host); err != nil {
				return nil, 0, internal(err)
			}
			return &host, 200, nil
		}
	}
	count, err := s.db.HostCount(ctx)
	if err != nil {
		return nil, 0, internal(err)
	}
	if count >= model.MaxHosts {
		return nil, 0, apiError(409, fmt.Sprintf("fleet limit reached (%d hosts)", model.MaxHosts))
	}
	host := model.Host{CapabilityReport: info.CapabilityReport, ImageSizes: info.ImageSizes, Capabilities: info.Capabilities, ID: info.HostID, Addr: request.Addr, Resource: info.Resource, State: model.HostOnline, Images: info.Images, Engines: info.Engines, Storage: info.Storage, RegisteredAt: s.now()}
	if err := s.db.PutHost(ctx, &host); err != nil {
		return nil, 0, internal(err)
	}
	return &host, 201, nil
}
func (s *Server) removeHost(ctx context.Context, id string) *APIError {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	vms, err := s.db.VMsByHost(ctx, id)
	if err != nil {
		return internal(err)
	}
	if len(vms) > 0 {
		return apiError(409, fmt.Sprintf("host %s still has %d VMs; destroy them first", id, len(vms)))
	}
	return internal(s.db.RemoveHost(ctx, id))
}
func (s *Server) detachHost(ctx context.Context, id string) ([]model.Vm, *APIError) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	host, err := s.db.GetHost(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	if host == nil {
		return nil, apiError(404, "host not found")
	}
	if host.State != model.HostOffline {
		return nil, apiError(409, "only an offline host can be detached; drain online guests first")
	}
	vms, err := s.db.VMsByHost(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	for _, vm := range vms {
		if s.busy(vm.EnvID) {
			return nil, apiError(409, "host has an environment operation in progress")
		}
	}
	orphans, err := s.db.DetachHost(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	s.revision.Add(1)
	return orphans, nil
}
func (s *Server) environmentDetail(ctx context.Context, id string) (api.EnvDetail, *APIError) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	env, err := s.db.GetEnv(ctx, id)
	if err != nil {
		return api.EnvDetail{}, internal(err)
	}
	if env == nil {
		return api.EnvDetail{}, apiError(404, "environment not found: "+id)
	}
	vms, err := s.db.VMsByEnv(ctx, id)
	if err != nil {
		return api.EnvDetail{}, internal(err)
	}
	warnings := model.List[string]{}
	for _, vm := range vms {
		if vm.Error != nil {
			warnings = append(warnings, vm.ID+": "+*vm.Error)
		}
	}
	if env.Error != nil {
		warnings = append(warnings, *env.Error)
	}
	return api.EnvDetail{Env: *env, VMs: vms, Warnings: warnings}, nil
}
