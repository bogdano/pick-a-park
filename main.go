package main

import (
	"encoding/json"
	"go-htmx/api"
	"go-htmx/components"
	_ "go-htmx/migrations"
	"go-htmx/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/cron"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	app := pocketbase.New()
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the
		// admin UI (isGoRun check is to enable it only during development)
		Automigrate: isGoRun,
	})

	// serves static files from the provided public dir (if exists)
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		template.NewTemplateRenderer(e.Router)

		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("/pb/pb_public"), false))

		e.Router.GET("/", func(c echo.Context) error {
			parks := []api.Park{}
			placeName := ""
			stateName := ""
			return template.Html(c, components.Index(parks, placeName, stateName))
		})

		e.Router.GET("/park/:parkCode", func(c echo.Context) error {
			parkCode := c.PathParam("parkCode") // use Param to get path parameters
			queryName := c.QueryParam("q")

			var placeRecord *models.Record
			// Proceed only if queryName is provided
			if queryName != "" {
				var err error
				placeRecord, err = app.Dao().FindFirstRecordByData("places", "placeName", queryName)
				if err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "Place not found"})
				}
			}
			var parkPlaceRecord *models.Record // Define outside to check later

			// regardless of queryParams, proceed to fetch park data
			parkRecord, err := app.Dao().FindFirstRecordByData("nationalParks", "parkCode", parkCode)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Park not found"})
			}
			if parkRecord != nil {
				var park api.Park
				park.FullName = parkRecord.GetString("name")
				park.Description = parkRecord.GetString("description")
				park.States = parkRecord.GetString("states")
				park.Images = parkRecord.Get("images").([]string)
				park.Longitude = parkRecord.GetString("longitude")
				park.Latitude = parkRecord.GetString("latitude")
				park.WeatherInfo = parkRecord.GetString("weatherInfo")
				park.DirectionsInfo = parkRecord.GetString("directionsInfo")
				park.ParkRecordId = parkRecord.Id
				park.ParkCode = parkCode
				var weatherData []api.WeatherDate
				err := json.Unmarshal([]byte(parkRecord.GetString("weather")), &weatherData)
				if err != nil {
					log.Println("Error unmarshaling JSON:", err)
				}
				park.Weather = weatherData
				parkPlaceRecord, err = app.Dao().FindFirstRecordByData("placeParks", "park", parkRecord.Id)
				if err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "Park not found"})
				}
				if parkPlaceRecord != nil {
					park.DriveTime = parkPlaceRecord.GetString("driveTime")
					park.DrivingDistance = parkPlaceRecord.GetString("drivingDistance")
				}

				placeName := ""
				if placeRecord != nil {
					placeName = placeRecord.GetString("placeName")
				}

				return template.Html(c, components.Park(park, placeName))
			} else {
				// Redirect to home page if park not found
				return c.Redirect(http.StatusFound, "/")
			}
		})

		e.Router.GET("/place/:placeName/:stateName", func(c echo.Context) error {
			placeName := c.PathParam("placeName")
			stateName := c.PathParam("stateName")
			queryName := placeName + "," + stateName
			// check if placeName is already in collection "places" under field "placeName"
			placeRecord, _ := app.Dao().FindFirstRecordByData("places", "placeName", queryName)
			if placeRecord != nil {
				// get the parks associated with the place
				placeParks, err := app.Dao().FindRecordsByExpr("placeParks", dbx.HashExp{"place": placeRecord.Id})
				if err != nil {
					return err
				}
				parks := []api.Park{}
				for _, placePark := range placeParks {
					parkId := placePark.GetStringSlice("park")[0]
					parkRecord, err := app.Dao().FindRecordById("nationalParks", parkId)
					if err != nil {
						return c.String(http.StatusInternalServerError, err.Error())
					}
					var park api.Park
					park.FullName = parkRecord.GetString("name")
					park.Description = parkRecord.GetString("description")
					park.States = parkRecord.GetString("states")
					park.Images = parkRecord.Get("images").([]string)
					park.Longitude = parkRecord.GetString("longitude")
					park.Latitude = parkRecord.GetString("latitude")
					park.ParkRecordId = parkRecord.Id
					park.DriveTime = placePark.GetString("driveTime")
					park.DrivingDistance = placePark.GetString("drivingDistance")
					park.ParkCode = parkRecord.GetString("parkCode")
					parks = append(parks, park)
				}
				// return all info from DB
				c.Response().Header().Set("HX-Push-Url", "/place/"+placeName+"/"+stateName)
				return template.Html(c, components.Index(parks, placeName, stateName))
			} else {
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
					park.States = record.GetString("states")
					park.Images = record.Get("images").([]string)
					park.Longitude = record.GetString("longitude")
					park.Latitude = record.GetString("latitude")
					park.ParkRecordId = record.Id
					park.ParkCode = record.GetString("parkCode")
					parks = append(parks, park)
				}
				// if not, add it with latitide and longitude and associate it with closest national parks
				// then, create entries in collection "placeParks" with the place and nationalPark, drivingDistance, driveTime
				// if placeName is already in collection, skip fetchDrivingDistances and return the parks associated with the place
				longitude, err := strconv.ParseFloat(c.FormValue("longitude"), 64)
				if err != nil {
					return c.String(http.StatusBadRequest, "Invalid longitude value")
				}
				latitude, err := strconv.ParseFloat(c.FormValue("latitude"), 64)
				if err != nil {
					return c.String(http.StatusBadRequest, "Invalid latitude value")
				}
				// create place record
				places, err := app.Dao().FindCollectionByNameOrId("places")
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				placeRecord = models.NewRecord(places)
				placeRecord.Set("placeName", queryName)
				placeRecord.Set("longitude", longitude)
				placeRecord.Set("latitude", latitude)
				if err := app.Dao().SaveRecord(placeRecord); err != nil {
					return err
				}
				// Fetch driving distances
				parks, err = api.FetchDrivingDistances([2]float64{latitude, longitude}, parks)
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				// save parks closest to this new place, with driving distances and time
				placeParks, err := app.Dao().FindCollectionByNameOrId("placeParks")
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				for _, park := range parks {
					placePark := models.NewRecord(placeParks)
					placePark.Set("place", placeRecord.Id)
					placePark.Set("park", park.ParkRecordId)
					placePark.Set("drivingDistance", park.DrivingDistance)
					placePark.Set("driveTime", park.DriveTime)
					if err := app.Dao().SaveRecord(placePark); err != nil {
						return err
					}
				}
				c.Response().Header().Set("HX-Push-Url", "/place/"+placeName+"/"+stateName)
				return template.Html(c, components.Index(parks, placeName, stateName))
			}
		})

		// route to fetch parks, commented because Pocketbase scheduler is set up to fetch parks every week
		e.Router.GET("/update-park-data", api.FetchAndStoreNationalParks(app))
		// route to fetch weather data
		e.Router.GET("/update-weather-data", api.FetchAndStoreWeather(app))

		// Start a cron that fetches and stores National Parks data once a week
		scheduler := cron.New()
		scheduler.MustAdd("updateParks", "0 0 * * 0", func() {
			log.Println("Fetching and storing National Parks data...")
			api.FetchAndStoreNationalParks(app)
			log.Println("National Parks data fetched and stored!")
		})
		// update weather data every 4 hours
		scheduler.MustAdd("updateWeather", "0 */4 * * *", func() {
			log.Println("Fetching and storing weather data...")
			api.FetchAndStoreWeather(app)
			log.Println("Weather data fetched and stored!")
		})
		scheduler.Start()
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
