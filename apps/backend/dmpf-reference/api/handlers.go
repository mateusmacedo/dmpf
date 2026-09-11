package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/orders"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const (
	// maxBodyBytes bounds what a write reads: the contract carries a SKU and a
	// quantity, and anything larger is not this API's request.
	maxBodyBytes = 1 << 16

	// maxSKULength matches the contract (AddItemRequest.sku maxLength).
	maxSKULength = 128
)

// orderIDFormat is the shape of {id} the contract publishes (OrderID pattern):
// the client mints the identifier, and the edge bounds what it accepts before
// it becomes a primary key and a partition key.
var orderIDFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type handlers struct{ service ordersapp.Service }

type addItemRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type itemAccepted struct {
	Order string `json:"order"`
	Items int    `json:"items"`
}

type placedResponse struct {
	Order string `json:"order"`
}

type itemView struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type orderView struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	ItemLimit int        `json:"itemLimit"`
	Items     []itemView `json:"items"`
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h handlers) addItem(w http.ResponseWriter, r *http.Request) {
	id, ok := orderIDOf(w, r)
	if !ok {
		return
	}
	var req addItemRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(w, http.StatusBadRequest, "malformed-body", "the body is not the AddItemRequest of the contract")
		return
	}
	if req.SKU == "" || len(req.SKU) > maxSKULength || req.Quantity < 1 {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "sku must have 1 to 128 characters and quantity must be at least 1")
		return
	}

	out, err := h.service.AddItem(r.Context(), ordersapp.AddItem{
		Order:    id,
		SKU:      orders.SKU(req.SKU),
		Quantity: req.Quantity,
	})
	if err != nil {
		writeFailure(w, err)
		return
	}
	if rej, refused := out.Rejection(); refused {
		writeDomainRejection(w, rej)
		return
	}
	writeJSON(w, http.StatusCreated, itemAccepted{Order: string(out.Response().Order), Items: out.Response().Items})
}

func (h handlers) placeOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := orderIDOf(w, r)
	if !ok {
		return
	}
	out, err := h.service.PlaceOrder(r.Context(), ordersapp.PlaceOrder{Order: id})
	if err != nil {
		writeFailure(w, err)
		return
	}
	if rej, refused := out.Rejection(); refused {
		writeDomainRejection(w, rej)
		return
	}
	writeJSON(w, http.StatusOK, placedResponse{Order: string(out.Response().Order)})
}

func (h handlers) findOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := orderIDOf(w, r)
	if !ok {
		return
	}
	snapshot, err := h.service.FindOrder(r.Context(), id)
	if err != nil {
		writeFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewOf(snapshot))
}

// orderIDOf reads {id} and refuses what the contract does not publish; false
// means the refusal was already written.
func orderIDOf(w http.ResponseWriter, r *http.Request) (orders.OrderID, bool) {
	id := r.PathValue("id")
	if !orderIDFormat.MatchString(id) {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "id must have 1 to 128 characters of [A-Za-z0-9._:-]")
		return "", false
	}
	return orders.OrderID(id), true
}

// decodeBody reads exactly one JSON object of the contract: unknown fields
// and anything after the object are refused, so a body the contract does not
// describe never reaches the service half-parsed.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("api: trailing data after the request body")
	}
	return nil
}

func viewOf(snapshot orders.Snapshot) orderView {
	items := make([]itemView, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		items = append(items, itemView{SKU: string(item.SKU), Quantity: item.Quantity})
	}
	return orderView{ID: string(snapshot.ID), Status: statusOf(snapshot.Status), ItemLimit: snapshot.ItemLimit, Items: items}
}

func statusOf(status orders.Status) string {
	if status == orders.Placed {
		return "placed"
	}
	return "open"
}

// writeFailure maps the technical channel: the two sentinels the ports
// declare get their own status, and everything else is 500 without detail
// (ERR-20). A classified failure keeps its status even when wrapped.
func writeFailure(w http.ResponseWriter, err error) {
	failure, classified := errors.AsType[*dmpfapplication.Failure](err)
	switch {
	case errors.Is(err, dmpfports.ErrNotFound), classified && failure.Category() == dmpfapplication.NotFound:
		writeRejection(w, http.StatusNotFound, "not-found", "no order with this id")
	case errors.Is(err, dmpfports.ErrVersionConflict), classified && failure.Category() == dmpfapplication.Conflict:
		writeRejection(w, http.StatusConflict, "version-conflict", "a concurrent write advanced the order; replay the request")
	default:
		writeRejection(w, http.StatusInternalServerError, "internal-failure", "the request could not be completed")
	}
}

func writeDomainRejection(w http.ResponseWriter, rej *dmpfdomain.Rejection) {
	writeRejection(w, http.StatusUnprocessableEntity, string(rej.Code()), rej.Message())
}

func writeRejection(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, rejection{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
