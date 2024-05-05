package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/labstack/echo/v5"
	"github.com/nfnt/resize"
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
	Images            []string `json:"-"` // "-" tells the json package to ignore this field when marshaling
	Designation       string   `json:"designation"`
	ParkCode          string   `json:"parkCode"`
	DriveTime         string
	DrivingDistance   string
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
		// decode the JSON response
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
				})
				form.RemoveFiles("images")
				for _, image := range park.Images {
					imageURL := image.URL
					// resize the image using my helper function
					resizedImageBytes, err := downloadAndResizeImage(imageURL, 1200)
					if err != nil {
						log.Printf("Error resizing image: %v", err)
						continue
					}
					// save the image to a temporary file
					tmpFile, err := filesystem.NewFileFromBytes(resizedImageBytes, path.Base(imageURL))
					if err != nil {
						log.Printf("Error saving image to a temporary file: %v", err)
						continue
					}
					log.Printf("Image size: %f kb", float64(tmpFile.Size)/1024.0)
					// add the image to the form
					form.AddFiles("images", tmpFile)
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

// downloadAndResizeImage downloads an image from the given URL and resizes it if it's larger than a maximum width.
func downloadAndResizeImage(url string, maxWidth uint) ([]byte, error) {
	// get the image from the NPS API URL
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	// decode the downloaded image
	img, _, err := image.Decode(response.Body)
	if err != nil {
		return nil, err
	}
	// resize the image if it's wider than the maxWidth
	if img.Bounds().Dx() > int(maxWidth) {
		img = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
	}
	// encode the image back to a byte slice as JPEG
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
