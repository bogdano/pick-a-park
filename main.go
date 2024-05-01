package main

import (
	"go-htmx/api"
	"go-htmx/components"
	"go-htmx/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	app := pocketbase.New()

	// serves static files from the provided public dir (if exists)
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		template.NewTemplateRenderer(e.Router)

		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))

		e.Router.GET("/", func(c echo.Context) error {
			return template.Html(c, components.Index())
		})

		e.Router.POST("/fetch-parks", func(c echo.Context) error {
			longitude, err := strconv.ParseFloat(c.FormValue("longitude"), 64)
			if err != nil {
				return c.String(http.StatusBadRequest, "Invalid longitude value")
			}
			latitude, err := strconv.ParseFloat(c.FormValue("latitude"), 64)
			if err != nil {
				return c.String(http.StatusBadRequest, "Invalid latitude value")
			}
			placeName := c.FormValue("placeName")
			placeName = strings.Split(placeName, ",")[0] + ", " + strings.Split(placeName, ",")[1]
			// get all records from nationalParks collection
			records, err := app.Dao().FindRecordsByExpr("nationalParks", nil)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			// Convert records to Park structs
			var parks []api.Park
			for _, record := range records {
				var park api.Park
				park.FullName = record.GetString("name")
				park.Description = record.GetString("description")
				park.Longitude = record.GetString("longitude")
				park.Latitude = record.GetString("latitude")
				parks = append(parks, park)
			}

			// Fetch driving distances
			parks, err = api.FetchDrivingDistances([2]float64{latitude, longitude}, parks)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			return template.Html(c, components.Parks(parks, placeName))
		})

		// route to fetch parks, commented because Pocketbase scheduler is set up to fetch parks every day
		// e.Router.GET("/fetchParks", api.FetchAndStoreNationalParks(app))

		// Start a cron that fetches and stores National Parks data every day
		scheduler := cron.New()
		scheduler.MustAdd("updateParks", "0 1 * * *", func() {
			api.FetchAndStoreNationalParks(app)
		})
		scheduler.Start()
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
