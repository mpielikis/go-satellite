package satellite

import (
	"errors"
	"math"
	"time"
)

// this procedure converts the day of the year, epochDays, to the equivalent month day, hour, minute and second.
func days2mdhms(year int64, epochDays float64) (mon, day, hr, min, sec float64) {
	lmonth := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	if year%4 == 0 {
		lmonth = [12]int{31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	}

	dayofyr := math.Floor(epochDays)

	i := 1.0
	inttemp := 0.0

	for dayofyr > inttemp+float64(lmonth[int(i-1)]) && i < 22 {
		inttemp = inttemp + float64(lmonth[int(i-1)])
		i += 1
	}

	mon = i
	day = dayofyr - inttemp

	temp := (epochDays - dayofyr) * 24.0
	hr = math.Floor(temp)

	temp = (temp - hr) * 60.0
	min = math.Floor(temp)

	sec = (temp - min) * 60.0

	return
}

func NewJDayFromTime(t time.Time) JDay {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	return NewJDay(year, int(month), day, hour, min, float64(sec))
}

// Calc julian date given year, month, day, hour, minute and second
// the julian date is defined by each elapsed day since noon, jan 1, 4713 bc.
func NewJDay(year, mon, day, hr, minute int, sec float64) JDay {
	jd := (367.0*float64(year) - math.Floor(7*(float64(year)+math.Floor((float64(mon)+9)/12.0))*0.25) + math.Floor(275*float64(mon)/9.0) + float64(day) + 1721013.5)
	fr := (sec + float64(minute)*60.0 + float64(hr)*3600.0) / 86400.0
	return JDay{jd, fr}
}

// this function finds the greenwich sidereal time (iau-82)
func gstime(jdut1 float64) (temp float64) {
	tut1 := (jdut1 - 2451545.0) / 36525.0
	temp = -6.2e-6*tut1*tut1*tut1 + 0.093104*tut1*tut1 + (876600.0*3600+8640184.812866)*tut1 + 67310.54841
	temp = math.Mod((temp * DEG2RAD / 240.0), TWOPI)

	if temp < 0.0 {
		temp += TWOPI
	}

	return
}

// Calc GST given year, month, day, hour, minute and second
func GSTimeFromDate(year, mon, day, hr, min int, sec float64) float64 {
	jDay := NewJDay(year, mon, day, hr, min, sec)
	return gstime(jDay.Single())
}

// Convert Earth Centered Inertial coordinated into equivalent latitude, longitude, altitude and velocity.
// Reference: http://celestrak.com/columns/v02n03/
func ECIToLLA(eciCoords Vector3, gmst float64) (altitude, velocity float64, ret LatLong) {
	a := 6378.137     // Semi-major Axis
	b := 6356.7523142 // Semi-minor Axis
	f := (a - b) / a  // Flattening
	e2 := ((2 * f) - math.Pow(f, 2))

	sqx2y2 := math.Sqrt(math.Pow(eciCoords.X, 2) + math.Pow(eciCoords.Y, 2))

	// Spherical Earth Calculations
	longitude := math.Atan2(eciCoords.Y, eciCoords.X) - gmst
	latitude := math.Atan2(eciCoords.Z, sqx2y2)

	// Oblate Earth Fix
	C := 0.0
	for i := 0; i < 20; i++ {
		C = 1 / math.Sqrt(1-e2*(math.Sin(latitude)*math.Sin(latitude)))
		latitude = math.Atan2(eciCoords.Z+(a*C*e2*math.Sin(latitude)), sqx2y2)
	}

	// Calc Alt
	altitude = (sqx2y2 / math.Cos(latitude)) - (a * C)

	// Orbital Speed ≈ sqrt(μ / r) where μ = std. gravitaional parameter
	velocity = math.Sqrt(398600.4418 / (altitude + 6378.137))

	ret.Latitude = latitude
	ret.Longitude = longitude

	return
}

// Convert LatLong in radians to LatLong in degrees
func LatLongDeg(rad LatLong) (deg LatLong, err error) {
	deg.Longitude = math.Mod(rad.Longitude/math.Pi*180, 360)
	if deg.Longitude > 180 {
		deg.Longitude = 360 - deg.Longitude
	} else if deg.Longitude < -180 {
		deg.Longitude = 360 + deg.Longitude
	}

	if rad.Latitude < (-math.Pi/2) || rad.Latitude > math.Pi/2 {
		err = errors.New("Latitude not within bounds -pi/2 to +pi/2")
		return
	}
	deg.Latitude = (rad.Latitude / math.Pi * 180)
	return
}

// Calculate GMST from Julian date.
// Reference: The 1992 Astronomical Almanac, page B6.
func ThetaG_JD(jday float64) (ret float64) {
	_, UT := math.Modf(jday + 0.5)
	jday = jday - UT
	TU := (jday - 2451545.0) / 36525.0
	GMST := 24110.54841 + TU*(8640184.812866+TU*(0.093104-TU*6.2e-6))
	GMST = math.Mod(GMST+86400.0*1.00273790934*UT, 86400.0)
	ret = 2 * math.Pi * GMST / 86400.0
	return
}

// Convert latitude, longitude and altitude into equivalent Earth Centered Intertial coordinates
// Reference: The 1992 Astronomical Almanac, page K11.
func LLAToECI(obsCoords LatLongAlt, jday float64, gravConst GravConst) (eciObs Vector3) {
	theta := math.Mod(ThetaG_JD(jday)+obsCoords.LatLong.Longitude, TWOPI)
	latSin := math.Sin(obsCoords.LatLong.Latitude)
	latCos := math.Cos(obsCoords.LatLong.Latitude)
	c := 1 / math.Sqrt(1+gravConst.f*(gravConst.f-2)*latSin*latSin)
	sq := c * (1 - gravConst.f) * (1 - gravConst.f)
	achcp := (gravConst.radiusearthkm*c + obsCoords.AltitudeKm) * latCos

	eciObs.X = achcp * math.Cos(theta)
	eciObs.Y = achcp * math.Sin(theta)
	eciObs.Z = (gravConst.radiusearthkm*sq + obsCoords.AltitudeKm) * latSin
	return
}

// Convert Earth Centered Intertial coordinates into Earth Cenetered Earth Final coordinates
// Reference: http://ccar.colorado.edu/ASEN5070/handouts/coordsys.doc
func ECIToECEF(eciCoords Vector3, gmst float64) (ecfCoords Vector3) {
	ecfCoords.X = eciCoords.X*math.Cos(gmst) + eciCoords.Y*math.Sin(gmst)
	ecfCoords.Y = eciCoords.X*-math.Sin(gmst) + eciCoords.Y*math.Cos(gmst)
	ecfCoords.Z = eciCoords.Z
	return
}

// Calculate look angles for given satellite position and observer position
// obsAlt in km
// Reference: http://celestrak.com/columns/v02n02/
func ECIToLookAngles(eciSat Vector3, obsCoords LatLongAlt, jday float64, gravConst GravConst) (lookAngles LookAngles) {
	theta := math.Mod(ThetaG_JD(jday)+obsCoords.LatLong.Longitude, 2*math.Pi)
	obsPos := LLAToECI(obsCoords, jday, gravConst)

	rx := eciSat.X - obsPos.X
	ry := eciSat.Y - obsPos.Y
	rz := eciSat.Z - obsPos.Z

	latSin := math.Sin(obsCoords.LatLong.Latitude)
	latCos := math.Cos(obsCoords.LatLong.Latitude)
	thetaSin := math.Sin(theta)
	thetaCos := math.Cos(theta)

	topS := latSin*thetaCos*rx + latSin*thetaSin*ry - latCos*rz
	topE := -thetaSin*rx + thetaCos*ry
	topZ := latCos*thetaCos*rx + latCos*thetaSin*ry + latSin*rz

	lookAngles.Az = math.Atan(-topE / topS)
	if topS > 0 {
		lookAngles.Az = lookAngles.Az + math.Pi
	}
	if lookAngles.Az < 0 {
		lookAngles.Az = lookAngles.Az + 2*math.Pi
	}
	lookAngles.Rg = math.Sqrt(rx*rx + ry*ry + rz*rz)
	lookAngles.El = math.Asin(topZ / lookAngles.Rg)

	return
}
