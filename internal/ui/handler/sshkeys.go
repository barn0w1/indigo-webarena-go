package handler

import (
	"fmt"
	"net/http"
	"strconv"

	indigo "github.com/barn0w1/indigo-webarena-go"
)

type sshKeyFormData struct {
	Key   *indigo.SSHKey
	Error string
}

func (h *Handler) handleSSHKeyList(w http.ResponseWriter, r *http.Request) {
	keys, err := h.client.SSH.List(r.Context())
	if err != nil {
		h.render(w, "sshkeys.html", pageData{
			Title:   "SSH Keys",
			Section: "sshkeys",
			Flash:   &flash{Kind: "err", Message: "Failed to load SSH keys: " + err.Error()},
			Data:    []indigo.SSHKey{},
		})
		return
	}
	h.render(w, "sshkeys.html", pageData{
		Title:   "SSH Keys",
		Section: "sshkeys",
		Data:    keys,
	})
}

func (h *Handler) handleSSHKeyNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, "sshkeys_new.html", pageData{
		Title:   "Add SSH Key",
		Section: "sshkeys",
		Data:    sshKeyFormData{},
	})
}

func (h *Handler) handleSSHKeyCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	pubKey := r.FormValue("publicKey")

	_, err := h.client.SSH.Create(r.Context(), indigo.CreateSSHKeyRequest{
		Name:      name,
		PublicKey: pubKey,
	})
	if err != nil {
		h.render(w, "sshkeys_new.html", pageData{
			Title:   "Add SSH Key",
			Section: "sshkeys",
			Flash:   &flash{Kind: "err", Message: "Failed to create SSH key: " + err.Error()},
			Data:    sshKeyFormData{Error: err.Error()},
		})
		return
	}

	http.Redirect(w, r, "/sshkeys", http.StatusSeeOther)
}

func (h *Handler) handleSSHKeyView(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	key, err := h.client.SSH.Get(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<tr id="sshkey-%d"><td colspan="4" class="error-cell">%s</td></tr>`, id, err.Error())
		return
	}
	h.renderPartial(w, "sshkey-row", key)
}

func (h *Handler) handleSSHKeyEdit(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	key, err := h.client.SSH.Get(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<tr><td colspan="4" class="error-cell">Failed to load key: %s</td></tr>`, err.Error())
		return
	}

	h.renderPartial(w, "sshkey-form", sshKeyFormData{Key: key})
}

func (h *Handler) handleSSHKeyUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	statusVal := r.FormValue("status")
	req := indigo.UpdateSSHKeyRequest{
		Name:   r.FormValue("name"),
		Status: indigo.SSHKeyStatus(statusVal),
	}

	if err := h.client.SSH.Update(r.Context(), id, req); err != nil {
		key := &indigo.SSHKey{
			ID:     id,
			Name:   r.FormValue("name"),
			Status: indigo.SSHKeyStatus(statusVal),
		}
		h.renderPartial(w, "sshkey-form", sshKeyFormData{Key: key, Error: err.Error()})
		return
	}

	key, err := h.client.SSH.Get(r.Context(), id)
	if err != nil {
		key = &indigo.SSHKey{
			ID:     id,
			Name:   r.FormValue("name"),
			Status: indigo.SSHKeyStatus(statusVal),
		}
	}

	h.renderPartial(w, "sshkey-row", key)
	h.renderPartial(w, "flash-oob", &flash{Kind: "ok", Message: "SSH key updated."})
}

func (h *Handler) handleSSHKeyDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.client.SSH.Delete(r.Context(), id); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<tr><td colspan="4" class="error-cell">Delete failed: %s</td></tr>`, err.Error())
		h.renderPartial(w, "flash-oob", &flash{Kind: "err", Message: "Delete failed: " + err.Error()})
		return
	}

	// Empty row replaces the deleted row (outerHTML swap).
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<tr style="display:none"></tr>`)
	h.renderPartial(w, "flash-oob", &flash{Kind: "ok", Message: strconv.Itoa(id) + " deleted."})
}
