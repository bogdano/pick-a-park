package main

import (
	"go-htmx/api"
	"go-htmx/components"
	_ "go-htmx/migrations"
	"go-htmx/template"
	"sort"

	"encoding/json"
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

		// set to ./pb_public when running locally
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("pb_public"), false))

		e.Router.GET("/", func(c echo.Context) error {
			parks := []api.Park{}
			placeName := ""
			stateName := ""
			return template.Html(c, components.Index(parks, placeName, stateName))
		})

		e.Router.GET("/offline", func(c echo.Context) error {
			return template.Html(c, components.Offline())
		})

		e.Router.GET("/park/:parkCode", func(c echo.Context) error {
			parkCode := c.PathParam("parkCode") // use Param to get path parameters
			queryName := c.QueryParam("q")

			var placeRecord *models.Record
			// Proceed only if queryName is provided
			if queryName != "" {
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
				park.Campgrounds = parkRecord.GetInt("campgrounds")

				var weatherData []api.WeatherDate
				err := json.Unmarshal([]byte(parkRecord.GetString("weather")), &weatherData)
				if err != nil {
					log.Println("Error unmarshaling JSON:", err)
				}
				park.Weather = weatherData

				var alerts []api.Alert
				alertRecords, err := app.Dao().FindRecordsByExpr("alerts", dbx.HashExp{"park": parkRecord.Id})
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				for _, alertRecord := range alertRecords {
					var alert api.Alert
					alert.Title = alertRecord.GetString("title")
					alert.Description = alertRecord.GetString("description")
					alert.Category = alertRecord.GetString("category")
					alert.Url = alertRecord.GetString("url")
					alerts = append(alerts, alert)
				}

				placeName := ""
				if queryName != "" {
					tmp, err := app.Dao().FindRecordsByExpr("placeParks", dbx.HashExp{"park": parkRecord.Id, "place": placeRecord.Id})
					if err != nil {
						return c.JSON(http.StatusBadRequest, map[string]string{"error": "Park not found"})
					}
					parkPlaceRecord = tmp[0]
					if parkPlaceRecord != nil {
						park.DriveTime = parkPlaceRecord.GetString("driveTime")
						park.DrivingDistance = parkPlaceRecord.GetString("drivingDistance")
					}
					if placeRecord != nil {
						placeName = placeRecord.GetString("placeName")
					}
				}
				return template.Html(c, components.Park(park, placeName, alerts))
			} else {
				// Redirect to home page if park not found
				return c.Redirect(http.StatusFound, "/")
			}
		})

		e.Router.GET("/campgrounds/:parkCode", func(c echo.Context) error {
			parkCode := c.PathParam("parkCode")
			parkRecord, err := app.Dao().FindFirstRecordByData("nationalParks", "parkCode", parkCode)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if parkRecord != nil {
				var campgrounds []api.Campground
				// get campgrounds associated with the park
				campgroundRecords, err := app.Dao().FindRecordsByExpr("campgrounds", dbx.HashExp{"parkId": parkRecord.Id})
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				for _, campgroundRecord := range campgroundRecords {
					var campground api.Campground
					campground.Name = campgroundRecord.GetString("name")
					if len(campgroundRecord.GetString("description")) > 100 {
						campground.Description = campgroundRecord.GetString("description")[0:100]
					} else {
						campground.Description = campgroundRecord.GetString("description")
					}
					campground.Latitude = campgroundRecord.GetString("latitude")
					campground.Longitude = campgroundRecord.GetString("longitude")
					campgrounds = append(campgrounds, campground)
				}
				park := api.Park{
					FullName:     parkRecord.GetString("name"),
					States:       parkRecord.GetString("states"),
					Longitude:    parkRecord.GetString("longitude"),
					Latitude:     parkRecord.GetString("latitude"),
					ParkRecordId: parkRecord.Id,
					ParkCode:     parkCode,
				}
				return template.Html(c, components.Campgrounds(park, campgrounds))
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
				placeParks = placeParks[:8]
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
				parks, err = api.FetchDrivingDistances([2]float64{latitude, longitude}, parks, 8)
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

		e.Router.GET("/load-more-parks/:placeName/:stateName", func(c echo.Context) error {
			placeName := c.PathParam("placeName")
			stateName := c.PathParam("stateName")
			currentCount, err := strconv.Atoi(c.QueryParam("currentCount"))
			if err != nil {
				return c.String(http.StatusBadRequest, "Invalid currentCount value")
			}
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
			// get all records from placeParks collection
			placeRecord, err := app.Dao().FindFirstRecordByData("places", "placeName", placeName+","+stateName)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			// get the parks already associated with the place
			placeParks, err := app.Dao().FindRecordsByExpr("placeParks", dbx.HashExp{"place": placeRecord.Id})
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			// sort placeParks by driving distance
			sort.Slice(placeParks, func(i, j int) bool {
				return placeParks[i].GetFloat("drivingDistance") < placeParks[j].GetFloat("drivingDistance")
			})
			// get the next 4 closest parks
			if len(placeParks) >= currentCount+4 {
				placeParks = placeParks[currentCount : currentCount+4]
				var newParks []api.Park
				for _, placePark := range placeParks {
					// if park is not in placeParks slice, remove it
					for _, park := range parks {
						if park.ParkRecordId == placePark.GetStringSlice("park")[0] {
							park.DrivingDistance = placePark.GetString("drivingDistance")
							park.DriveTime = placePark.GetString("driveTime")
							newParks = append(newParks, park)
						}
					}
				}
				return template.Html(c, components.MoreParks(newParks, placeName, stateName))
			} else {
				// remove current parks from the list, then get driving distances to next 4 closest parks
				var newParks []api.Park
				for _, park := range parks {
					found := false
					for _, placePark := range placeParks {
						if park.ParkRecordId == placePark.GetStringSlice("park")[0] {
							found = true
							break
						}
					}
					if !found {
						newParks = append(newParks, park)
					}
				}
				// Fetch driving distances
				newParks, err = api.FetchDrivingDistances([2]float64{placeRecord.GetFloat("latitude"), placeRecord.GetFloat("longitude")}, newParks, 4)
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				// save parks closest to this new place, with driving distances and time
				placeParks, err := app.Dao().FindCollectionByNameOrId("placeParks")
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				for _, park := range newParks {
					placePark := models.NewRecord(placeParks)
					placePark.Set("place", placeRecord.Id)
					placePark.Set("park", park.ParkRecordId)
					placePark.Set("drivingDistance", park.DrivingDistance)
					placePark.Set("driveTime", park.DriveTime)
					if err := app.Dao().SaveRecord(placePark); err != nil {
						return err
					}
				}
				return template.Html(c, components.MoreParks(newParks, placeName, stateName))
			}
		})

		// route to fetch parks, commented because Pocketbase scheduler is set up to fetch parks every week
		e.Router.GET("/update-park-data", api.FetchAndStoreNationalParksHTTP(app))
		// route to fetch weather data
		e.Router.GET("/update-weather-data", api.FetchAndStoreWeatherHTTP(app))
		// route to fetch alerts
		e.Router.GET("/update-alerts", api.FetchAlertsHTTP(app))

		// Start a cron that fetches and stores National Parks data once a week
		scheduler := cron.New()
		scheduler.MustAdd("updateParks", "0 0 * * 0", func() {
			log.Println("Fetching and storing National Parks data...")
			err := api.FetchAndStoreNationalParks(app)
			if err != nil {
				log.Println("Error fetching National Parks data:", err)
				return
			}
			log.Println("National Parks data fetched and stored!")
		})
		// update weather data every 4 hours
		scheduler.MustAdd("updateWeather", "0 */4 * * *", func() {
			log.Println("Fetching and storing weather data...")
			err := api.FetchAndStoreWeather(app)
			if err != nil {
				log.Println("Error fetching weather data:", err)
				return
			}
			log.Println("Weather data fetched and stored!")
		})
		// update alerts every 24 hours
		scheduler.MustAdd("updateAlerts", "0 0 * * *", func() {
			log.Println("Fetching and storing alerts data...")
			err := api.FetchAlerts(app)
			if err != nil {
				log.Println("Error fetching alerts data:", err)
				return
			}
			log.Println("Alerts data fetched and stored!")
		})
		scheduler.Start()
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
