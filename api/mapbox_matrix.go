package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
)

// FetchDrivingDistances fetches driving distances using the Mapbox Matrix API and sorts by Haversine distance.
func FetchDrivingDistances(startCoordinates [2]float64, parksData []Park) ([]Park, error) {
	// Calculate Haversine distance for each park and sort
	for i := range parksData {
		latitude, _ := strconv.ParseFloat(parksData[i].Latitude, 64)
		longitude, _ := strconv.ParseFloat(parksData[i].Longitude, 64)
		parkCoords := [2]float64{latitude, longitude}
		parksData[i].HaversineDistance = haversineDistance(startCoordinates, parkCoords, true)
	}
	sort.Slice(parksData, func(i, j int) bool {
		return parksData[i].HaversineDistance < parksData[j].HaversineDistance
	})
	// Select the top 6 closest parks
	if len(parksData) > 6 {
		parksData = parksData[:6]
	}

	// Mapbox Matrix API call for driving distances
	mapboxAccessToken := os.Getenv("MAPBOX_ACCESS_TOKEN")
	// Construct the coordinates part of the URL
	coordinates := fmt.Sprintf("%f,%f", startCoordinates[1], startCoordinates[0]) // Starting point
	for _, park := range parksData {
		coordinates += ";" + park.Longitude + "," + park.Latitude // Destination points
	}
	// Construct the full URL with all parameters
	url := fmt.Sprintf("https://api.mapbox.com/directions-matrix/v1/mapbox/driving/%s?sources=0&annotations=distance&access_token=%s", coordinates, mapboxAccessToken)

	// Make a GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch data: %s", resp.Status)
	}

	var response struct {
		Distances [][]float64 `json:"distances"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	// Attach driving distances to parks
	for i := range parksData {
		parksData[i].DrivingDistance = response.Distances[0][i+1] // +1 to skip the start location
	}
	return parksData, nil
}

// Haversine formula for calculating distances between two coordinates
func haversineDistance(coords1, coords2 [2]float64, isMiles bool) float64 {
	const R = 6371.0 // Radius of the Earth in kilometers
	dLat := toRad(coords2[0] - coords1[0])
	dLon := toRad(coords2[1] - coords1[1])
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(coords1[0]))*math.Cos(toRad(coords2[0]))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := R * c
	if isMiles {
		return distance * 0.621371
	}
	return distance
}

func toRad(deg float64) float64 {
	return deg * (math.Pi / 180)
}
