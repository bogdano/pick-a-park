package api

import (
	"os"
	"strings"
	"log"

	"github.com/pocketbase/pocketbase"
)

func GenerateSitemap(app *pocketbase.PocketBase) (error) {
	// Generate sitemap.xml
	var sitemap string
	sitemap += `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	sitemap += `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n"
	sitemap += `<url><loc>https://pick-a-park.com/</loc></url>` + "\n"
	parks, err := app.Dao().FindRecordsByExpr("nationalParks")
	if err != nil {
		log.Printf("Error fetching national parks: %v", err)
		return err
	}
	for _, park := range parks {
		sitemap += `<url><loc>https://pick-a-park.com/park/` + park.GetString("parkCode") + `</loc></url>` + "\n"
		sitemap += `<url><loc>https://pick-a-park.com/campgrounds/` + park.GetString("parkCode") + `</loc></url>` + "\n"
	}
	campgrounds, err := app.Dao().FindRecordsByExpr("campgrounds")
	if err != nil {
		log.Printf("Error fetching campgrounds: %v", err)
		return err
	}
	for _, campground := range campgrounds {
		sitemap += `<url><loc>https://pick-a-park.com/campground/` + campground.GetString("campId") + `</loc></url>` + "\n"
	}
	// get unique places from placeParks collection
	places, err := app.Dao().FindRecordsByExpr("uniquePlaces")
	if err != nil {
		log.Printf("Error fetching places: %v", err)
		return err
	}
	for _, place := range places {
		placeName := strings.Split(place.GetString("placeName"), ",")
		// placeName slice may contain spaces which need to be URL encoded
		placeName[0], placeName[1] = strings.ReplaceAll(placeName[0], " ", "%20"), strings.ReplaceAll(placeName[1], " ", "%20")
		sitemap += `<url><loc>https://pick-a-park.com/place/` + placeName[0] + `/` + placeName[1] + `</loc></url>` + "\n"
	}

	sitemap += `</urlset>`
	// create sitemap.xml file in pb_public
	err = os.WriteFile("pb_public/sitemap.xml", []byte(sitemap), 0644)
	if err != nil {
		log.Printf("Error writing sitemap.xml: %v", err)
		return err
	}
	return nil
}
