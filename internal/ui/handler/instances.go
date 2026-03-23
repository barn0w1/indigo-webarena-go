package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	indigo "github.com/barn0w1/indigo-webarena-go"
)

type osItem struct {
	ID   int
	Name string
}

type createFormData struct {
	Types   []indigo.InstanceType
	Regions []indigo.Region
	OS      []osItem
	Specs   []indigo.InstanceSpec
	Keys    []indigo.SSHKey
	Error   string
}

func (h *Handler) handleInstanceList(w http.ResponseWriter, r *http.Request) {
	instances, err := h.client.Instance.List(r.Context())
	if err != nil {
		h.render(w, "instances.html", pageData{
			Title:   "Instances",
			Section: "instances",
			Flash:   &flash{Kind: "err", Message: "Failed to load instances: " + err.Error()},
			Data:    []indigo.Instance{},
		})
		return
	}
	h.render(w, "instances.html", pageData{
		Title:   "Instances",
		Section: "instances",
		Data:    instances,
	})
}

func (h *Handler) handleInstanceNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var (
		types   []indigo.InstanceType
		regions []indigo.Region
		osCats  []indigo.OSCategory
		specs   []indigo.InstanceSpec
		keys    []indigo.SSHKey
		mu      sync.Mutex
		errs    []error
		wg      sync.WaitGroup
	)

	do := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}

	do(func() error { var err error; types, err = h.client.Instance.ListTypes(ctx); return err })
	do(func() error { var err error; regions, err = h.client.Instance.ListRegions(ctx, 0); return err })
	do(func() error { var err error; osCats, err = h.client.Instance.ListOS(ctx, 0); return err })
	do(func() error { var err error; specs, err = h.client.Instance.ListSpecs(ctx, 0, 0); return err })
	do(func() error { var err error; keys, err = h.client.SSH.List(ctx); return err })
	wg.Wait()

	if len(errs) > 0 {
		h.render(w, "instance_create.html", pageData{
			Title:   "New Instance",
			Section: "instances",
			Flash:   &flash{Kind: "err", Message: "Failed to load form data: " + errs[0].Error()},
			Data:    createFormData{},
		})
		return
	}

	h.render(w, "instance_create.html", pageData{
		Title:   "New Instance",
		Section: "instances",
		Data: createFormData{
			Types:   types,
			Regions: regions,
			OS:      bestEffortOS(osCats),
			Specs:   specs,
			Keys:    keys,
		},
	})
}

func (h *Handler) handleInstanceCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	planStr := r.FormValue("plan")
	plan, _ := strconv.Atoi(planStr)

	req := indigo.CreateInstanceRequest{
		Name:        name,
		Plan:        plan,
		WinPassword: r.FormValue("winPassword"),
	}

	if v := r.FormValue("regionId"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			req.RegionID = &n
		}
	}
	if v := r.FormValue("osId"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			req.OSID = &n
		}
	}
	if v := r.FormValue("sshKeyId"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			req.SSHKeyID = &n
		}
	}

	inst, err := h.client.Instance.Create(r.Context(), req)
	if err != nil {
		h.render(w, "instance_create.html", pageData{
			Title:   "New Instance",
			Section: "instances",
			Flash:   &flash{Kind: "err", Message: "Failed to create instance: " + err.Error()},
			Data:    createFormData{},
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/instances/%d", inst.ID), http.StatusSeeOther)
}

func (h *Handler) handleInstanceDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	instances, err := h.client.Instance.List(r.Context())
	if err != nil {
		h.render(w, "instance_detail.html", pageData{
			Title:   "Instance",
			Section: "instances",
			Flash:   &flash{Kind: "err", Message: "Failed to load instance: " + err.Error()},
		})
		return
	}

	inst, ok := findInstance(instances, id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	h.render(w, "instance_detail.html", pageData{
		Title:   inst.Name,
		Section: "instances",
		Data:    inst,
	})
}

func (h *Handler) makeInstanceAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instanceID := r.PathValue("id")
		result, err := h.client.Instance.UpdateStatus(r.Context(), instanceID, indigo.InstanceAction(action))
		if err != nil {
			h.errFlash(w, fmt.Sprintf("Action %q failed: %s", action, err.Error()))
			return
		}
		msg := result.Message
		if msg == "" {
			msg = fmt.Sprintf("Action %q applied (status: %s)", action, result.InstanceStatus)
		}
		h.okFlash(w, msg)
	}
}

func findInstance(instances []indigo.Instance, id int) (*indigo.Instance, bool) {
	for i := range instances {
		if instances[i].ID == id {
			return &instances[i], true
		}
	}
	return nil, false
}

func bestEffortOS(cats []indigo.OSCategory) []osItem {
	items := make([]osItem, 0, len(cats))
	for i, c := range cats {
		var item struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		if json.Unmarshal(c.Raw, &item) == nil && item.Name != "" {
			items = append(items, osItem{ID: item.ID, Name: item.Name})
		} else {
			items = append(items, osItem{ID: i, Name: fmt.Sprintf("OS %d", i+1)})
		}
	}
	return items
}
