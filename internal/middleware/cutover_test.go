package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	"github.com/noah-isme/sma-adp-api/pkg/config"
)

func TestCutoverStageMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.CutoverConfig{RouteToGo: true, StageHeader: "X-Stage", ClientSegmentHeader: "X-Segment"}
	svc := service.NewCutoverService(cfg, nil)

	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(CutoverStage(svc))
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if got := recorder.Header().Get("X-Stage"); got != "canary" {
		t.Fatalf("unexpected stage header: %s", got)
	}
	if got := recorder.Header().Get("X-Segment"); got == "" {
		t.Fatalf("expected segment header to be set")
	}
}

func TestCutoverMetadataExtraction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(cutoverStageContextKey, models.CutoverStageShadow)
	c.Set(cutoverSegmentContextKey, "segment-01")

	stage, segment := CutoverMetadata(c)
	if stage != models.CutoverStageShadow {
		t.Fatalf("unexpected stage: %s", stage)
	}
	if segment != "segment-01" {
		t.Fatalf("unexpected segment: %s", segment)
	}
}
