package handlers

import (
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"unicode/utf8"

	"sifeucr/database"
	"sifeucr/internal/browser"
	"sifeucr/internal/httpx"
	"sifeucr/middleware"
	"sifeucr/routes"
)

type supplierRequest struct {
	IdentityNumber string `json:"identity_number"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	IBAN           string `json:"iban"`
	BudgetID       int64  `json:"budget_id"`
}

func (h *Handler) registerSupplierRoutes() {
	h.AddRoute(routes.Suppliers, middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, http.HandlerFunc(h.listSuppliers)))
	h.AddRoute(routes.CreateSupplier, h.adminOnly(h.saveSupplier))
	h.AddRoute(routes.UpdateSupplier, h.adminOnly(h.saveSupplier))
}

func (h *Handler) listSuppliers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Querier().ListSuppliers(r.Context())
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.suppliers",
			"failed to list suppliers",
		)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rows)
}

func (h *Handler) saveSupplier(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[supplierRequest](h, w, r, "err.invalid_supplier", "invalid supplier request")
	if !ok {
		return
	}
	s := database.Supplier{
		IdentityNumber: strings.TrimSpace(req.IdentityNumber), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description),
		Email: strings.TrimSpace(req.Email), Phone: strings.TrimSpace(req.Phone), IBAN: strings.TrimSpace(req.IBAN), BudgetID: req.BudgetID,
	}
	address, err := mail.ParseAddress(s.Email)
	validText := func(value string, limit int) bool { return value != "" && utf8.RuneCountInString(value) <= limit }
	if validText(s.IdentityNumber, 64) && validPanelName(s.Name) && validText(s.Description, 4000) &&
		err == nil && address.Address == s.Email && len(s.Email) <= 254 && validText(s.Phone, 64) && validText(s.IBAN, 64) && s.BudgetID > 0 {
		q := h.db.Querier()
		write := q.CreateSupplier
		status := http.StatusCreated
		if r.Method == http.MethodPut {
			s.ID, err = strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil || s.ID <= 0 {
				h.writeStatus(
					w, r, http.StatusBadRequest, "err.invalid_supplier", "invalid supplier ID",
				)
				return
			}
			write = q.UpdateSupplier
			status = http.StatusNoContent
		}
		n, err := write(r.Context(), s)
		if err != nil {
			h.writeError(
				w,
				r,
				http.StatusInternalServerError,
				err,
				"err.suppliers",
				"failed to save supplier",
			)
			return
		}
		if n == 1 {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(status)
			return
		}
	}
	h.writeStatus(
		w,
		r,
		http.StatusBadRequest,
		"err.invalid_supplier",
		"invalid supplier details or budget",
	)
}

func (h *Handler) browserSaveSupplier(w http.ResponseWriter, r *http.Request, cookies browser.Cookies) {
	id, err := strconv.ParseInt(r.PostForm.Get("budget_id"), 10, 64)
	if err != nil {
		id = 0
	}
	payload := supplierRequest{
		IdentityNumber: r.PostForm.Get("identity_number"), Name: r.PostForm.Get("name"), Description: r.PostForm.Get("description"),
		Email: r.PostForm.Get("email"), Phone: r.PostForm.Get("phone"), IBAN: r.PostForm.Get("iban"), BudgetID: id,
	}
	request := r.Clone(r.Context())
	expectedStatus := http.StatusCreated
	if r.PostForm.Get("action") == "update_supplier" {
		request.Method = http.MethodPut
		request.SetPathValue("id", r.PostForm.Get("supplier_id"))
		expectedStatus = http.StatusNoContent
	}
	reply := browser.Call(request, h.adminOnly(h.saveSupplier), payload, cookies.Read(r, "access"))
	if reply.Status != expectedStatus {
		browserForward(w, reply)
		return
	}
	if browserJSON(r) {
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"refreshSuppliers": true})
		return
	}
	myURL, _ := h.URL(routes.MyPage)
	http.Redirect(w, r, myURL+"#suppliers", http.StatusSeeOther)
}
