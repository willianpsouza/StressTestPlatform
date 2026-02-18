package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/middleware"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/influxdb"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/postgres"
	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type InfluxDBHandler struct {
	influxClient *influxdb.Client
	testRepo     *postgres.TestRepository
}

func NewInfluxDBHandler(influxClient *influxdb.Client, testRepo *postgres.TestRepository) *InfluxDBHandler {
	return &InfluxDBHandler{influxClient: influxClient, testRepo: testRepo}
}

func (h *InfluxDBHandler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	// Get user's tests to find their buckets
	filter := domain.TestFilter{
		Pagination: domain.Pagination{Page: 1, PageSize: 100},
	}
	if claims.Role != domain.UserRoleRoot {
		filter.UserID = &claims.UserID
	}

	tests, _, err := h.testRepo.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	type bucketInfo struct {
		Name     string `json:"name"`
		TestName string `json:"test_name"`
		TestID   string `json:"test_id"`
	}

	buckets := make([]bucketInfo, 0, len(tests))
	for _, t := range tests {
		buckets = append(buckets, bucketInfo{
			Name:     t.InfluxDBBucket,
			TestName: t.Name,
			TestID:   t.ID.String(),
		})
	}

	response.OK(w, buckets)
}

func (h *InfluxDBHandler) ClearBucket(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	bucketName := chi.URLParam(r, "name")

	if bucketName == "" {
		response.BadRequest(w, "Bucket name is required")
		return
	}

	// Verify the user owns a test with this bucket (unless ROOT)
	if claims.Role != domain.UserRoleRoot {
		filter := domain.TestFilter{
			Pagination: domain.Pagination{Page: 1, PageSize: 100},
			UserID:     &claims.UserID,
		}
		tests, _, err := h.testRepo.List(filter)
		if err != nil {
			response.Error(w, err)
			return
		}
		found := false
		for _, t := range tests {
			if t.InfluxDBBucket == bucketName {
				found = true
				break
			}
		}
		if !found {
			response.Error(w, domain.NewForbiddenError("You do not own a test with this bucket"))
			return
		}
	}

	if err := h.influxClient.ClearBucket(bucketName); err != nil {
		response.Error(w, domain.NewAppError("INFLUXDB_ERROR", "Failed to clear bucket: "+err.Error(), http.StatusInternalServerError))
		return
	}

	response.OK(w, map[string]string{"message": "Bucket data cleared"})
}
