package schedule

import (
	"math"
	"time"
)

// The Prudential Tower.
const (
	TowerLat = 42.3471
	TowerLon = -71.0825
)

// Sunset returns when the sun sets at the given place on a calendar date,
// using NOAA's solar position equations (good to about a minute at Boston's
// latitude). ok is false when the sun doesn't set that day.
func Sunset(year int, month time.Month, day int, lat, lon float64) (t time.Time, ok bool) {
	// work from solar noon in UTC on that date
	jd := julianDay(year, month, day)
	// two passes: compute at noon, then refine at the first estimate
	minutes := 720.0 - 4*lon
	for range 2 {
		jc := julianCentury(jd + minutes/1440.0)
		eqTime := equationOfTime(jc)
		decl := sunDeclination(jc)
		ha, okHA := hourAngleSunset(lat, decl)
		if !okHA {
			return time.Time{}, false
		}
		minutes = 720 - 4*(lon-ha) - eqTime
	}
	base := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(minutes * float64(time.Minute))).Round(time.Minute), true
}

func julianDay(year int, month time.Month, day int) float64 {
	y, m := year, int(month)
	if m <= 2 {
		y--
		m += 12
	}
	a := math.Floor(float64(y) / 100)
	b := 2 - a + math.Floor(a/4)
	return math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + float64(day) + b - 1524.5
}

func julianCentury(jd float64) float64 { return (jd - 2451545.0) / 36525.0 }

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

func geomMeanLongSun(t float64) float64 {
	l := math.Mod(280.46646+t*(36000.76983+t*0.0003032), 360)
	if l < 0 {
		l += 360
	}
	return l
}

func geomMeanAnomalySun(t float64) float64 { return 357.52911 + t*(35999.05029-0.0001537*t) }

func eccentricityEarthOrbit(t float64) float64 { return 0.016708634 - t*(0.000042037+0.0000001267*t) }

func sunEqOfCenter(t float64) float64 {
	m := rad(geomMeanAnomalySun(t))
	return math.Sin(m)*(1.914602-t*(0.004817+0.000014*t)) + math.Sin(2*m)*(0.019993-0.000101*t) + math.Sin(3*m)*0.000289
}

func sunApparentLong(t float64) float64 {
	o := geomMeanLongSun(t) + sunEqOfCenter(t)
	omega := 125.04 - 1934.136*t
	return o - 0.00569 - 0.00478*math.Sin(rad(omega))
}

func obliquityCorrection(t float64) float64 {
	seconds := 21.448 - t*(46.815+t*(0.00059-t*0.001813))
	e0 := 23 + (26+seconds/60)/60
	omega := 125.04 - 1934.136*t
	return e0 + 0.00256*math.Cos(rad(omega))
}

func sunDeclination(t float64) float64 {
	return deg(math.Asin(math.Sin(rad(obliquityCorrection(t))) * math.Sin(rad(sunApparentLong(t)))))
}

func equationOfTime(t float64) float64 {
	eps := obliquityCorrection(t)
	l0 := geomMeanLongSun(t)
	e := eccentricityEarthOrbit(t)
	m := geomMeanAnomalySun(t)
	y := math.Pow(math.Tan(rad(eps)/2), 2)
	et := y*math.Sin(2*rad(l0)) - 2*e*math.Sin(rad(m)) + 4*e*y*math.Sin(rad(m))*math.Cos(2*rad(l0)) -
		0.5*y*y*math.Sin(4*rad(l0)) - 1.25*e*e*math.Sin(2*rad(m))
	return deg(et) * 4 // minutes
}

// hourAngleSunset is the sun's hour angle at sunset in degrees, with the
// standard -0.833° for refraction and the sun's radius.
func hourAngleSunset(lat, decl float64) (float64, bool) {
	cos := math.Cos(rad(90.833))/(math.Cos(rad(lat))*math.Cos(rad(decl))) - math.Tan(rad(lat))*math.Tan(rad(decl))
	if cos < -1 || cos > 1 {
		return 0, false
	}
	return deg(math.Acos(cos)), true
}
