package services

import (
	"math"
	"testing"

	"tripledger/types"
)

func TestDistanceKmUsesGreatCircleDistance(t *testing.T) {
	distance := distanceKm(0, 0, 0, 1)
	if math.Abs(distance-111.195) > 0.01 {
		t.Fatalf("distanceKm(0, 0, 0, 1) = %.3f, want about 111.195 km", distance)
	}
}

func TestWithinNearbyRadius(t *testing.T) {
	inside := distanceKm(0, 0, 0, 0.179)
	outside := distanceKm(0, 0, 0, 0.181)
	if !withinNearbyRadius(inside) {
		t.Fatalf("%.2f km should be within the 20 km suggestion radius", inside)
	}
	if withinNearbyRadius(outside) {
		t.Fatalf("%.2f km should be outside the 20 km suggestion radius", outside)
	}
}

func TestValidateTripLocations(t *testing.T) {
	latitude, longitude := 19.033, 73.029
	stops := []types.TripStopInput{{
		Name:      "Panvel",
		Latitude:  &latitude,
		Longitude: &longitude,
	}}

	if err := validateTripLocations("Mumbai", &latitude, &longitude, "Pune", &latitude, &longitude, stops); err != nil {
		t.Fatalf("valid route rejected: %v", err)
	}
	if err := validateTripLocations("Mumbai", nil, &longitude, "Pune", &latitude, &longitude, nil); err == nil {
		t.Fatal("route without a selected start location was accepted")
	}
	invalidLatitude := 91.0
	if err := validateTripLocations("Mumbai", &invalidLatitude, &longitude, "Pune", &latitude, &longitude, nil); err == nil {
		t.Fatal("out-of-range coordinates were accepted")
	}

	manyStops := make([]types.TripStopInput, 21)
	for i := range manyStops {
		manyStops[i] = stops[0]
	}
	if err := validateTripLocations("Mumbai", &latitude, &longitude, "Pune", &latitude, &longitude, manyStops); err == nil {
		t.Fatal("route with more than 20 stops was accepted")
	}
}
