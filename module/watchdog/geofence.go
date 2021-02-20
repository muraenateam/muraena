package watchdog

import (
	"regexp"

	"github.com/kellydunn/golang-geo"
)

var geofenceRegexp = regexp.MustCompile(`^([-+]?[0-9]*\.?[0-9]+)[^-+0-9]+([-+]?[0-9]*\.?[0-9]+)(?:[^0-9]+([0-9]*\.?[0-9]+)([A-Za-z]*)[^0-9]*)?$`)
var geofenceUnits = map[string]float64{
	"":   1.0,
	"m":  1.0,
	"km": 1000.0,
	"mi": 1609.0,
	"ft": 1609.0 / 5280.0,
}

// Geofence represents a point on the Earth with an accuracy radius in meters.
type Geofence struct {
	Type                        GeofenceType
	Field                       string
	Value                       string
	Latitude, Longitude, Radius float64
}

type GeofenceType string

const (
	Location  GeofenceType = "Location"
	Parameter              = "Parameter"
)

// SetIntersection is a description of the relationship between two sets.
type SetIntersection uint

const (
	// IsDisjoint means that the two sets have no common elements.
	IsDisjoint SetIntersection = 1 << iota

	// IsSubset means the first set is a subset of the second.
	IsSubset

	// IsSuperset means the second set is a subset of the first.
	IsSuperset
)

// Intersection describes the relationship between two geofences
func (mi *Geofence) Intersection(tu *Geofence) (i SetIntersection) {
	miPoint := geo.NewPoint(mi.Latitude, mi.Longitude)
	tuPoint := geo.NewPoint(tu.Latitude, tu.Longitude)
	distance := miPoint.GreatCircleDistance(tuPoint) * 1000

	radiusSum := mi.Radius + tu.Radius
	radiusDiff := mi.Radius - tu.Radius

	if distance-radiusSum > 0 {
		i = IsDisjoint
		return
	}

	if -distance+radiusDiff >= 0 {
		i |= IsSuperset
	}

	if -distance-radiusDiff >= 0 {
		i |= IsSubset
	}

	return
}
