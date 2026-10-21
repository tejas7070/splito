package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const nearbyPlacesRadiusMeters = 20_000

type PlaceSuggestion struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Nearby     bool    `json:"nearby"`
	DistanceKm float64 `json:"distance_km,omitempty"`
}

type PlaceSearchResult struct {
	Suggestions       []PlaceSuggestion `json:"suggestions"`
	NearbyUnavailable bool              `json:"nearby_unavailable"`
}

type PlaceService struct {
	client       *http.Client
	nearbyClient *http.Client
}

func NewPlaceService() *PlaceService {
	return &PlaceService{
		client:       &http.Client{Timeout: 5 * time.Second},
		nearbyClient: &http.Client{Timeout: 3 * time.Second},
	}
}

type photonResponse struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Name     string `json:"name"`
			Street   string `json:"street"`
			Locality string `json:"locality"`
			District string `json:"district"`
			City     string `json:"city"`
			County   string `json:"county"`
			State    string `json:"state"`
			Country  string `json:"country"`
			OSMType  string `json:"osm_type"`
			OSMID    int64  `json:"osm_id"`
		} `json:"properties"`
	} `json:"features"`
}

type overpassResponse struct {
	Elements []struct {
		Type   string             `json:"type"`
		ID     int64              `json:"id"`
		Lat    float64            `json:"lat"`
		Lon    float64            `json:"lon"`
		Center map[string]float64 `json:"center"`
		Tags   map[string]string  `json:"tags"`
	} `json:"elements"`
}

func (s *PlaceService) Search(ctx context.Context, query string) (*PlaceSearchResult, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 3 || len([]rune(query)) > 100 {
		return nil, errors.New("search must be between 3 and 100 characters")
	}

	searchURL := "https://photon.komoot.io/api/?" + url.Values{
		"q":     []string{query},
		"limit": []string{"20"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create place search request: %w", err)
	}
	req.Header.Set("User-Agent", "Splito/1.0 (trip location suggestions)")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("place search provider unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("place search provider returned HTTP %d", resp.StatusCode)
	}

	var photon photonResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&photon); err != nil {
		return nil, fmt.Errorf("decode place search response: %w", err)
	}

	result := &PlaceSearchResult{Suggestions: make([]PlaceSuggestion, 0, len(photon.Features))}
	seen := make(map[string]struct{})
	var anchor *PlaceSuggestion
	for _, feature := range photon.Features {
		coordinates := feature.Geometry.Coordinates
		if len(coordinates) < 2 || !validCoordinates(coordinates[1], coordinates[0]) {
			continue
		}
		name := formatPlaceName(feature.Properties.Name, feature.Properties.Street, feature.Properties.Locality,
			feature.Properties.City, feature.Properties.District, feature.Properties.County,
			feature.Properties.State, feature.Properties.Country)
		if name == "" {
			continue
		}
		suggestion := PlaceSuggestion{
			ID:        fmt.Sprintf("%s/%d", feature.Properties.OSMType, feature.Properties.OSMID),
			Name:      name,
			Latitude:  coordinates[1],
			Longitude: coordinates[0],
		}
		if suggestion.ID == "/0" {
			suggestion.ID = fmt.Sprintf("%s/%.6f/%.6f", strings.ToLower(name), suggestion.Latitude, suggestion.Longitude)
		}
		appendUniquePlace(result, seen, suggestion)
		if anchor == nil {
			copy := suggestion
			anchor = &copy
		}
	}

	if anchor == nil {
		return result, nil
	}

	nearby, err := s.searchNearby(ctx, anchor.Latitude, anchor.Longitude)
	if err != nil {
		result.NearbyUnavailable = true
		return result, nil
	}
	sort.Slice(nearby, func(i, j int) bool { return nearby[i].DistanceKm < nearby[j].DistanceKm })
	for _, place := range nearby {
		appendUniquePlace(result, seen, place)
	}
	return result, nil
}

func (s *PlaceService) searchNearby(ctx context.Context, latitude, longitude float64) ([]PlaceSuggestion, error) {
	query := fmt.Sprintf(`[out:json][timeout:8];nwr["place"~"^(city|town|village|suburb|neighbourhood|hamlet|locality)$"](around:%d,%.7f,%.7f);out center tags;`,
		nearbyPlacesRadiusMeters, latitude, longitude)
	form := url.Values{"data": []string{query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://overpass-api.de/api/interpreter", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create nearby places request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Splito/1.0 (trip location suggestions)")
	resp, err := s.nearbyClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nearby places provider unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nearby places provider returned HTTP %d", resp.StatusCode)
	}

	var data overpassResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode nearby places response: %w", err)
	}

	places := make([]PlaceSuggestion, 0, len(data.Elements))
	for _, element := range data.Elements {
		name := strings.TrimSpace(element.Tags["name"])
		if name == "" {
			name = strings.TrimSpace(element.Tags["name:en"])
		}
		lat, lon := element.Lat, element.Lon
		if element.Type != "node" {
			var latitudeExists, longitudeExists bool
			lat, latitudeExists = element.Center["lat"]
			lon, longitudeExists = element.Center["lon"]
			if !latitudeExists || !longitudeExists {
				continue
			}
		}
		if name == "" || !validCoordinates(lat, lon) {
			continue
		}
		distance := distanceKm(latitude, longitude, lat, lon)
		if !withinNearbyRadius(distance) {
			continue
		}
		places = append(places, PlaceSuggestion{
			ID:         fmt.Sprintf("%s/%d", element.Type, element.ID),
			Name:       name,
			Latitude:   lat,
			Longitude:  lon,
			Nearby:     true,
			DistanceKm: math.Round(distance*10) / 10,
		})
	}
	return places, nil
}

func formatPlaceName(name string, parts ...string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	result := []string{name}
	seen := map[string]struct{}{strings.ToLower(name): {}}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, part)
	}
	return strings.Join(result, ", ")
}

func appendUniquePlace(result *PlaceSearchResult, seen map[string]struct{}, place PlaceSuggestion) {
	key := place.ID
	if key == "" {
		key = fmt.Sprintf("%s/%.6f/%.6f", strings.ToLower(strings.TrimSpace(place.Name)), place.Latitude, place.Longitude)
	}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	result.Suggestions = append(result.Suggestions, place)
}

func validCoordinates(latitude, longitude float64) bool {
	return !math.IsNaN(latitude) && !math.IsInf(latitude, 0) &&
		!math.IsNaN(longitude) && !math.IsInf(longitude, 0) &&
		latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0088
	toRadians := math.Pi / 180
	dLat := (lat2 - lat1) * toRadians
	dLon := (lon2 - lon1) * toRadians
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*toRadians)*math.Cos(lat2*toRadians)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func withinNearbyRadius(distance float64) bool {
	return distance <= float64(nearbyPlacesRadiusMeters)/1000
}
