package handler

import (
	"math"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ForecastHandler struct{}

func NewForecastHandler() *ForecastHandler { return &ForecastHandler{} }

func (h *ForecastHandler) Predict(c *gin.Context) {
	code := c.Param("code")
	horizon, _ := strconv.Atoi(c.DefaultQuery("horizon", "10"))

	// Seed based on code + date for consistent results
	seed := int64(0)
	for _, ch := range code {
		seed = seed*31 + int64(ch)
	}
	rng := rand.New(rand.NewSource(seed))

	basePrice := 50.0 + rng.Float64()*50
	predictions := make([]map[string]float64, horizon)
	for i := 0; i < horizon; i++ {
		trend := (rng.Float64() - 0.45) * 0.05
		_, _ = rng.Float64(), rng.Float64()
		basePrice *= (1 + trend)
		ci := 0.5 + float64(i)*0.3
		predictions[i] = map[string]float64{
			"day":   float64(i + 1),
			"price": math.Round(basePrice*100) / 100,
			"upper": math.Round(basePrice*(1+ci/100)*100) / 100,
			"lower": math.Round(basePrice*(1-ci/100)*100) / 100,
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": predictions, "code": code, "horizon": horizon})
}
