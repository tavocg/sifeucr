package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sifeucr/database"
	"sifeucr/internal/browser"
	"sifeucr/internal/httpx"
	"sifeucr/middleware"
	"sifeucr/routes"
)

type budgetRequest struct {
	Name       string    `json:"name"`
	ItemNumber string    `json:"item_number"`
	Amount     string    `json:"amount"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
}

type budgetResponse struct {
	ID int64 `json:"id"`
	budgetRequest
}

type processRequest struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Name     string    `json:"name"`
	BudgetID int64     `json:"budget_id"`
}

type processResponse struct {
	ID int64 `json:"id"`
	processRequest
	BudgetName string `json:"budget_name"`
}

func (h *Handler) adminOnly(next http.HandlerFunc) http.Handler {
	authorized := middleware.RequirePermission(h.logger, h.localize, h.db.Querier(), database.PermissionAdmin, next)
	return middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, authorized)
}

func (h *Handler) registerPanelRoutes() {
	h.AddRoute(routes.Budgets, h.adminOnly(h.listBudgets))
	h.AddRoute(routes.CreateBudget, h.adminOnly(h.createBudget))
	h.AddRoute(routes.PurchaseProcesses, h.adminOnly(h.listProcesses))
	h.AddRoute(routes.CreatePurchaseProcess, h.adminOnly(h.createProcess))
}

func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Querier().ListBudgets(r.Context())
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.panel",
			"failed to list budgets",
		)
		return
	}
	result := make([]budgetResponse, 0, len(rows))
	for _, b := range rows {
		result = append(result, budgetResponse{
			ID: b.ID, budgetRequest: budgetRequest{
				Name: b.Name, ItemNumber: b.ItemNumber, Amount: b.Amount(),
				StartsAt: time.Unix(b.StartsAt, 0).UTC(), EndsAt: time.Unix(b.EndsAt, 0).UTC(),
			},
		})
	}
	httpx.WriteJSON(w, 200, result)
}

func (h *Handler) listProcesses(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Querier().ListProcesses(r.Context())
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.panel",
			"failed to list purchase processes",
		)
		return
	}
	result := make([]processResponse, 0, len(rows))
	for _, p := range rows {
		result = append(result, processResponse{ID: p.ID, processRequest: processRequest{Name: p.Name, BudgetID: p.BudgetID, StartsAt: time.Unix(p.StartsAt, 0).UTC(), EndsAt: time.Unix(p.EndsAt, 0).UTC()}, BudgetName: p.BudgetName})
	}
	httpx.WriteJSON(w, 200, result)
}

// Parse decimal text directly so a budget never passes through floating point.
func amountCents(amount string) (int64, bool) {
	whole, fraction, decimal := strings.Cut(amount, ".")
	if whole == "" || decimal && (fraction == "" || len(fraction) > 2) {
		return 0, false
	}
	digits := whole + fraction
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	cents, err := strconv.ParseInt(digits+strings.Repeat("0", 2-len(fraction)), 10, 64)
	return cents, err == nil
}

func validItemNumber(item string) bool {
	for _, digit := range item {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return item != ""
}

func validPanelTime(t time.Time) bool {
	year := t.UTC().Year()
	return !t.IsZero() && t.Nanosecond() == 0 && year >= 1 && year <= 9999
}

func validPeriod(start, end time.Time) bool {
	return validPanelTime(start) && validPanelTime(end) && end.After(start)
}

func validPanelName(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= 160
}

func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[budgetRequest](h, w, r, "err.invalid_budget", "invalid budget request")
	if !ok {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	cents, ok := amountCents(req.Amount)
	if !ok || !validPanelName(req.Name) || !validItemNumber(req.ItemNumber) || !validPeriod(req.StartsAt, req.EndsAt) {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.invalid_budget",
			"invalid budget",
		)
		return
	}
	err := h.db.Querier().CreateBudget(r.Context(), database.Budget{Name: req.Name, ItemNumber: req.ItemNumber, StartsAt: req.StartsAt.Unix(), EndsAt: req.EndsAt.Unix(), AmountCents: cents})
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.panel",
			"failed to create budget",
		)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) createProcess(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[processRequest](h, w, r, "err.invalid_process", "invalid purchase process request")
	if !ok {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if validPanelName(req.Name) && req.BudgetID > 0 && validPeriod(req.StartsAt, req.EndsAt) {
		n, err := h.db.Querier().CreateProcess(r.Context(), database.PurchaseProcess{Name: req.Name, BudgetID: req.BudgetID, StartsAt: req.StartsAt.Unix(), EndsAt: req.EndsAt.Unix()})
		if err != nil {
			h.writeError(
				w,
				r,
				http.StatusInternalServerError,
				err,
				"err.panel",
				"failed to create purchase process",
			)
			return
		}
		if n == 1 {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusCreated)
			return
		}
	}
	h.writeStatus(
		w,
		r,
		http.StatusBadRequest,
		"err.invalid_process",
		"invalid purchase process",
	)
}

func (h *Handler) browserCreatePanelRecord(w http.ResponseWriter, r *http.Request, cookies browser.Cookies, action string) {
	var payload any
	var next http.HandlerFunc

	start, _ := time.Parse("2006-01-02T15:04", r.PostForm.Get("starts_at"))
	end, _ := time.Parse("2006-01-02T15:04", r.PostForm.Get("ends_at"))
	if action == "create_budget" {
		payload = budgetRequest{Name: r.PostForm.Get("name"), ItemNumber: r.PostForm.Get("item_number"), Amount: r.PostForm.Get("amount"), StartsAt: start, EndsAt: end}
		next = h.createBudget
	} else {
		id, err := strconv.ParseInt(r.PostForm.Get("budget_id"), 10, 64)
		if err != nil {
			id = 0
		}
		payload = processRequest{Name: r.PostForm.Get("name"), BudgetID: id, StartsAt: start, EndsAt: end}
		next = h.createProcess
	}
	reply := browser.Call(r, h.adminOnly(next), payload, cookies.Read(r, "access"))
	if reply.Status != http.StatusCreated {
		browserForward(w, reply)
		return
	}
	if browserJSON(r) {
		httpx.WriteJSON(w, 200, map[string]bool{"refreshPanel": true, "refreshSuppliers": action == "create_budget"})
		return
	}
	target, _ := h.URL(routes.MyPage)
	http.Redirect(w, r, target+"#panel", http.StatusSeeOther)
}
