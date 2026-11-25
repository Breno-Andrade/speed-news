package httpapi

import (
	"net/http"
)

func (apiServer *server) handleLastRace(responseWriter http.ResponseWriter, request *http.Request) {
	lastRace, err := apiServer.RaceInsights.LastRace(request.Context())
	apiServer.writeInsight(responseWriter, "lastRace", lastRace, err)
}

func (apiServer *server) handleCircuitProfile(responseWriter http.ResponseWriter, request *http.Request) {
	circuitProfile, err := apiServer.RaceInsights.CircuitProfile(request.Context())
	apiServer.writeInsight(responseWriter, "circuit", circuitProfile, err)
}

func (apiServer *server) handleWeekendForecast(responseWriter http.ResponseWriter, request *http.Request) {
	weekendForecast, err := apiServer.RaceInsights.WeekendForecast(request.Context())
	apiServer.writeInsight(responseWriter, "forecast", weekendForecast, err)
}

func (apiServer *server) writeInsight(responseWriter http.ResponseWriter, responseField string, insightPayload any, err error) {
	if err != nil {
		apiServer.Logger.Warn(responseField+" unavailable", "error", err)
		writeError(responseWriter, http.StatusBadGateway, "source unavailable, try again later")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{responseField: insightPayload})
}

func (apiServer *server) handleCurrentRaceWeekend(responseWriter http.ResponseWriter, request *http.Request) {
	raceWeekendOverview, err := apiServer.RaceWeekendService.CurrentOverview(request.Context())
	if err != nil {
		apiServer.writeInternalError(responseWriter, "current race weekend", err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]any{"raceWeekend": raceWeekendOverview})
}
