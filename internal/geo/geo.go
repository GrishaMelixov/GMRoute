package geo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Location struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Country string  `json:"country"`
}

var (
	cache  sync.Map
	client = &http.Client{Timeout: 2 * time.Second}
)

func Lookup(ip string) (*Location, error) {
	if v, ok := cache.Load(ip); ok {
		return v.(*Location), nil
	}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,lat,lon,country", ip))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r struct {
		Status  string  `json:"status"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		Country string  `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if r.Status != "success" {
		return nil, fmt.Errorf("geo lookup failed for %s", ip)
	}

	loc := &Location{Lat: r.Lat, Lng: r.Lon, Country: r.Country}
	cache.Store(ip, loc)
	return loc, nil
}

// LookupSelf returns the geographic location of the current machine.
func LookupSelf() (*Location, error) {
	resp, err := client.Get("http://ip-api.com/json/?fields=status,lat,lon,country")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r struct {
		Status  string  `json:"status"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		Country string  `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if r.Status != "success" {
		return nil, fmt.Errorf("self geo lookup failed")
	}
	return &Location{Lat: r.Lat, Lng: r.Lon, Country: r.Country}, nil
}
