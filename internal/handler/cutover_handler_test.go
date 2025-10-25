package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type stubCutoverService struct {
	legacyResult models.CutoverPingResult
	legacyErr    error
	goResult     models.CutoverPingResult
	goErr        error
}

func (s stubCutoverService) PingLegacy(context.Context) (models.CutoverPingResult, error) {
	return s.legacyResult, s.legacyErr
}

func (s stubCutoverService) PingGo(context.Context) (models.CutoverPingResult, error) {
	return s.goResult, s.goErr
}

func TestCutoverHandlerSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewCutoverHandler(stubCutoverService{
		goResult: models.CutoverPingResult{
			Target:    "go",
			Reachable: true,
			Stage:     models.CutoverStageCanary,
			Duration:  time.Millisecond,
		},
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/ping-go", nil)

	handler.PingGo(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestCutoverHandlerFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewCutoverHandler(stubCutoverService{
		legacyResult: models.CutoverPingResult{Target: "legacy"},
		legacyErr:    errors.New("unreachable"),
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/ping-legacy", nil)

	handler.PingLegacy(c)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if header := recorder.Header().Get("X-Cutover-Error"); header == "" {
		t.Fatalf("expected error header to be set")
	}
}
