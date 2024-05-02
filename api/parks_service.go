package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type Park struct {
	FullName          string   `json:"fullName"`
	Description       string   `json:"description"`
	Latitude          string   `json:"latitude"`
	Longitude         string   `json:"longitude"`
	States            string   `json:"states"`
	ImageURL          string   `json:"-"` // just save NPS url for an image as fallback
	Images            []string `json:"-"` // "-" tells the json package to ignore this field when marshaling
	Designation       string   `json:"designation"`
	ParkCode          string   `json:"parkCode"`
	DrivingDistance   float64
	HaversineDistance float64
	ParkRecordId      string
}

func FetchAndStoreNationalParks(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		// fetch data from NPS API
		NPS_API_KEY := os.Getenv("NPS_API_KEY")
		var NPS_API_URL = "https://developer.nps.gov/api/v1/parks?limit=500&api_key=" + NPS_API_KEY
		resp, err := http.Get(NPS_API_URL)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		defer resp.Body.Close()

		var data struct {
			Data []struct {
				Park
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Fatal(err)
		}
		// get the Pocketbase collection for National Parks
		collection, err := app.Dao().FindCollectionByNameOrId("nationalParks")
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		// filter for national parks only and store in Pocketbase
		for _, park := range data.Data {
			if park.Designation == "National Park" {
				var record *models.Record
				existingRecord, err := app.Dao().FindFirstRecordByData("nationalParks", "parkCode", park.ParkCode)
				if err == nil {
					record = existingRecord
					record.Set("images", nil)
				} else {
					record = models.NewRecord(collection)
					record.Set("parkCode", park.ParkCode)
				}

				// load regular data into the form
				form := forms.NewRecordUpsert(app, record)
				form.LoadData(map[string]any{
					"name":        park.FullName,
					"description": park.Description,
					"latitude":    park.Latitude,
					"longitude":   park.Longitude,
					"states":      park.States,
					"imageURL":    park.Images[0].URL,
				})

				for _, image := range park.Images {
					imageURL := image.URL
					// create a temp file to save the JPG image
					tmpFile, err := filesystem.NewFileFromUrl(context.TODO(), imageURL)
					if err != nil {
						log.Printf("Error creating temp file for image: %v", err)
						continue
					}
					if tmpFile.Size < 5242880 {
						// add the file to the form if not > 5mb
						form.AddFiles("images", tmpFile)
					} else {
						log.Printf("Image file is too large: %v, skipping...", tmpFile.Size)
					}
				}
				// validate and save the record with the image(s)
				if err := form.Submit(); err != nil {
					log.Printf("Error saving record with image: %v", err)
					continue
				}
				log.Printf("Record saved: %v", record.Id)
			}
		}
		return c.String(http.StatusOK, "National Parks data has been stored successfully.")
	}
}
