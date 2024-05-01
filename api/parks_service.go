package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

type Park struct {
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
	States      string `json:"states"`
	Images      []struct {
		URL string `json:"url"`
	} `json:"images"`
	Designation       string `json:"designation"`
	ParkCode          string `json:"parkCode"`
	DrivingDistance   float64
	HaversineDistance float64
}

func FetchAndStoreNationalParks(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		// fetch data from NPS API
		NPS_API_KEY := os.Getenv("NPS_API_KEY")
		var NPS_API_URL = "https://developer.nps.gov/api/v1/parks?limit=500&api_key=" + NPS_API_KEY

		resp, err := http.Get(NPS_API_URL)
		// print api url to console
		log.Println(NPS_API_URL)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		defer resp.Body.Close()

		var data struct {
			Data []Park `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		collection, err := app.Dao().FindCollectionByNameOrId("nationalParks")
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		// filter for national parks only and store in Pocketbase
		for _, park := range data.Data {
			if park.Designation == "National Park" {
				var record *models.Record
				var existingRecord *models.Record
				existingRecord, err = app.Dao().FindFirstRecordByData("nationalParks", "parkCode", park.ParkCode)
				if err == nil {
					record = existingRecord
				} else {
					record = models.NewRecord(collection)
					record.Set("parkCode", park.ParkCode)
				}
				record.Set("name", park.FullName)
				record.Set("description", park.Description)
				latitude, err := strconv.ParseFloat(park.Latitude, 64)
				if err != nil {
					log.Fatalf("Error parsing latitude: %v", err)
				}
				record.Set("latitude", latitude)
				longitude, err := strconv.ParseFloat(park.Longitude, 64)
				if err != nil {
					log.Fatalf("Error parsing longitude: %v", err)
				}
				record.Set("longitude", longitude)
				record.Set("states", park.States)
				record.Set("image", park.Images[0].URL)
				if err := app.Dao().SaveRecord(record); err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
			}
		}

		return c.String(http.StatusOK, "National Parks data has been stored successfully.")
	}
}
