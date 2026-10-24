package main

import (
	"context"
	"flag"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"

	treefarm "github.com/oapi-codegen/oapi-codegen/v2/examples/callback"
)

// TreeFarm implements treefarm.StrictServerInterface.
type TreeFarm struct {
	initiator *treefarm.CallbackInitiator
}

var _ treefarm.StrictServerInterface = (*TreeFarm)(nil)

func NewTreeFarm() *TreeFarm {
	initiator, err := treefarm.NewCallbackInitiator()
	if err != nil {
		log.Fatalf("Failed to create callback initiator: %v", err)
	}
	return &TreeFarm{initiator: initiator}
}

func plantTreeError(code int, message string) treefarm.PlantTreedefaultJSONResponse {
	return treefarm.PlantTreedefaultJSONResponse{
		StatusCode: code,
		Body: treefarm.Error{
			Code:    int32(code),
			Message: message,
		},
	}
}

func (tf *TreeFarm) PlantTree(ctx context.Context, request treefarm.PlantTreeRequestObject) (treefarm.PlantTreeResponseObject, error) {
	if request.Body == nil {
		return plantTreeError(http.StatusBadRequest, "request body is required"), nil
	}
	req := *request.Body

	if req.CallbackURL == "" {
		return plantTreeError(http.StatusBadRequest, "callbackUrl is required"), nil
	}

	id := uuid.New()

	log.Printf("Accepted tree planting: id=%s kind=%q location=%q callbackUrl=%q",
		id, req.Kind, req.Location, req.CallbackURL)

	go tf.plantAndNotify(id, req)

	return treefarm.PlantTree202JSONResponse{
		Location: req.Location,
		Kind:     req.Kind,
		ID:       id,
	}, nil
}

func (tf *TreeFarm) plantAndNotify(id uuid.UUID, req treefarm.TreePlantingRequest) {
	delay := time.Duration(1+rand.IntN(5)) * time.Second
	log.Printf("Planting tree %s (kind=%q, location=%q) — will complete in %s",
		id, req.Kind, req.Location, delay)

	time.Sleep(delay)

	result := treefarm.TreePlantedJSONRequestBody{
		ID:       id,
		Kind:     req.Kind,
		Location: req.Location,
		Success:  true,
	}

	log.Printf("Tree %s planted, invoking callback at %s", id, req.CallbackURL)

	resp, err := tf.initiator.TreePlanted(context.Background(), req.CallbackURL, result)
	if err != nil {
		log.Printf("Callback to %s failed: %v", req.CallbackURL, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	log.Printf("Callback to %s returned status %d", req.CallbackURL, resp.StatusCode)
}

func main() {
	port := flag.String("port", "8080", "Port for HTTP server")
	flag.Parse()

	farm := NewTreeFarm()

	mux := http.NewServeMux()
	treefarm.HandlerFromMux(treefarm.NewStrictHandler(farm, nil), mux)

	// Wrap with request logging.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		mux.ServeHTTP(w, r)
	})

	addr := net.JoinHostPort("0.0.0.0", *port)
	log.Printf("Tree Farm server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
